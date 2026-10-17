package proxies

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// connectProxy is an HTTP proxy answering plain requests with its name,
// and opening CONNECT tunnels that greet with its name and the host asked
// for, then echo.
func connectProxy(t *testing.T, name string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			_, _ = io.WriteString(w, name)
			return
		}
		conn, brw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\n\r\n"+name+" "+r.Host+"\n")
		_, _ = io.Copy(conn, brw)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// tunnelThrough opens a CONNECT tunnel to target through the gateway at
// addr, returning the first line read from it.
func tunnelThrough(t *testing.T, addr, user, password, target string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	req := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n"
	if user != "" {
		req += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password)) + "\r\n"
	}
	if _, err := io.WriteString(conn, req+"\r\n"); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	return conn, br, resp.StatusCode
}

func TestGateway(t *testing.T) {
	s := newService(t, nil)
	ctx := t.Context()
	office := connectProxy(t, "office")
	p, err := s.Create(ctx, httpProxy("office", office))
	if err != nil {
		t.Fatal(err)
	}
	choices := map[string]string{"dev.mavio.indexers": p.ID.String(), "mavio-solvers-flaresolverr": ""}
	g := NewGateway(s, []byte("key"), func(_ context.Context, user string) (string, bool) {
		id, ok := choices[user]
		return id, ok
	}, nil)
	t.Cleanup(g.Close)
	srv := httptest.NewServer(g.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "server")
	})))
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	user, pw := "dev.mavio.indexers", g.Password("dev.mavio.indexers")
	home := connectProxy(t, "home")

	t.Run("refused", func(t *testing.T) {
		for _, tc := range []struct {
			name, user, password string
			want                 int
		}{
			{"no credentials", "", "", http.StatusProxyAuthRequired},
			{"wrong password", user, "x", http.StatusProxyAuthRequired},
			{"another user's password", "mavio-solvers-flaresolverr", pw, http.StatusProxyAuthRequired},
			{"no longer a user", "gone", g.Password("gone"), http.StatusForbidden},
		} {
			if _, _, got := tunnelThrough(t, addr, tc.user, tc.password, "indexer.test:443"); got != tc.want {
				t.Errorf("%s: status got = %d, want = %d", tc.name, got, tc.want)
			}
		}
	})

	t.Run("tunnel through the chosen proxy", func(t *testing.T) {
		conn, br, status := tunnelThrough(t, addr, user, pw, "indexer.test:443")
		if status != http.StatusOK {
			t.Fatalf("status got = %d, want = 200", status)
		}
		if line, _ := br.ReadString('\n'); line != "office indexer.test:443\n" {
			t.Errorf("greeting got = %q, want = office indexer.test:443", line)
		}
		_, _ = io.WriteString(conn, "ping\n")
		if line, _ := br.ReadString('\n'); line != "ping\n" {
			t.Errorf("echo got = %q, want = ping", line)
		}

		// A changed proxy ends the tunnels, for them to go through it.
		changed := httpProxy("office", home)
		changed.ID = p.ID
		if _, err := s.Update(ctx, changed, false); err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, err := br.ReadByte(); err == nil {
			t.Error("tunnel read after the proxy changed got = no error, want = closed")
		}
		_, br, _ = tunnelThrough(t, addr, user, pw, "indexer.test:443")
		if line, _ := br.ReadString('\n'); line != "home indexer.test:443\n" {
			t.Errorf("greeting after the change got = %q, want = home indexer.test:443", line)
		}
	})

	t.Run("local targets directly", func(t *testing.T) {
		target := connectProxy(t, "local")
		u := "mavio-solvers-flaresolverr"
		conn, br, status := tunnelThrough(t, addr, u, g.Password(u), target.Listener.Addr().String())
		if status != http.StatusOK {
			t.Fatalf("status got = %d, want = 200", status)
		}
		_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n")
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "local" {
			t.Errorf("body got = %q, want = local", body)
		}
	})

	t.Run("plain HTTP forwarded", func(t *testing.T) {
		pu, _ := url.Parse("http://" + addr)
		pu.User = url.UserPassword(user, pw)
		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
		resp, err := client.Get("http://api.test/3")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "home" {
			t.Errorf("body got = %q, want = home", body)
		}
		// Requests to the server itself are served as ever.
		resp, err = http.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if body, _ := io.ReadAll(resp.Body); string(body) != "server" {
			t.Errorf("server body got = %q, want = server", body)
		}
	})
}

func TestGatewayEnv(t *testing.T) {
	g := NewGateway(newService(t, nil), []byte("key"), nil, nil)
	env := g.Env("dev.mavio.agent", "host.docker.internal:8686", "mavio-agent-db")
	want := "http://dev.mavio.agent:" + g.Password("dev.mavio.agent") + "@host.docker.internal:8686"
	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	for _, k := range []string{"HTTP_PROXY", "https_proxy"} {
		if got[k] != want {
			t.Errorf("%s got = %q, want = %q", k, got[k], want)
		}
	}
	no := strings.Split(got["no_proxy"], ",")
	for _, h := range []string{"localhost", "192.168.0.0/16", "host.docker.internal", "mavio-agent-db"} {
		if !slices.Contains(no, h) {
			t.Errorf("no_proxy got = %v, want %s in it", no, h)
		}
	}
	if v, ok := got["ALL_PROXY"]; !ok || v != "" {
		t.Errorf("ALL_PROXY got = %q, %v, want = empty", v, ok)
	}
}

func TestRemoteLocal(t *testing.T) {
	for remote, want := range map[string]bool{
		"127.0.0.1:5000": true, "[::1]:5000": true, "172.17.0.2:5000": true, "192.168.65.1:5000": true,
		"203.0.113.9:5000": false, "[2001:db8::1]:5000": false, "garbage": false,
	} {
		if got := remoteLocal(remote); got != want {
			t.Errorf("remoteLocal(%q) got = %v, want = %v", remote, got, want)
		}
	}
}
