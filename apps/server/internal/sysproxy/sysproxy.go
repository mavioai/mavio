package sysproxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Setting is a system proxy.
type Setting struct {
	// URL is the proxy, http, socks4 or socks5, with its credentials; nil
	// when none is set.
	URL *url.URL
	// Bypass lists the hosts reached directly, as NO_PROXY does, with
	// CIDR blocks, "*" wildcards and "<local>" for names without a dot.
	Bypass []string
}

// Detect returns the system proxy: the environment's HTTPS_PROXY,
// HTTP_PROXY or ALL_PROXY, in this order, with NO_PROXY, or else the
// operating system's: the network settings on macOS, the Internet
// Options on Windows.
func Detect(ctx context.Context) (Setting, error) {
	if s, ok, err := FromEnv(os.Getenv); ok || err != nil {
		return s, err
	}
	return fromOS(ctx)
}

// FromEnv reads the system proxy from environment variables, upper or
// lower case; ok is whether one is set.
func FromEnv(getenv func(string) string) (s Setting, ok bool, err error) {
	get := func(name string) string {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
		return strings.TrimSpace(getenv(strings.ToLower(name)))
	}
	for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY"} {
		v := get(name)
		if v == "" {
			continue
		}
		u, err := parse(v, "http")
		if err != nil {
			return Setting{}, true, fmt.Errorf("%s: %w", name, err)
		}
		return Setting{URL: u, Bypass: split(get("NO_PROXY"), ",")}, true, nil
	}
	return Setting{}, false, nil
}

// parse reads a proxy as an environment variable or the operating system
// gives it, of the scheme when it has none.
func parse(v, scheme string) (*url.URL, error) {
	if !strings.Contains(v, "://") {
		v = scheme + "://" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy %q: %w", v, err)
	}
	port := "1080"
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme, port = "http", "80"
	case "socks5", "socks5h":
		u.Scheme = "socks5"
	case "socks4", "socks4a":
		u.Scheme = "socks4"
	default:
		return nil, fmt.Errorf("proxy %q: unsupported scheme %q", u.Redacted(), u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("proxy %q has no host", u.Redacted())
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), port)
	}
	if _, err := strconv.ParseUint(u.Port(), 10, 16); err != nil {
		return nil, fmt.Errorf("proxy %q has an invalid port", u.Redacted())
	}
	return &url.URL{Scheme: u.Scheme, User: u.User, Host: u.Host}, nil
}

func split(v, sep string) []string {
	var out []string
	for e := range strings.SplitSeq(v, sep) {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// ParseScutil reads the system proxy from the output of macOS's
// scutil --proxy: its HTTPS, HTTP or SOCKS proxy, in this order.
func ParseScutil(out []byte) (Setting, error) {
	values := map[string]string{}
	var exceptions []string
	inExceptions := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, found := strings.Cut(line, " : ")
		switch {
		case line == "}":
			inExceptions = false
		case !found:
		case inExceptions:
			exceptions = append(exceptions, v)
		case k == "ExceptionsList":
			inExceptions = true
		default:
			values[k] = v
		}
	}
	var s Setting
	for _, p := range []struct{ prefix, scheme string }{{"HTTPS", "http"}, {"HTTP", "http"}, {"SOCKS", "socks5"}} {
		host, port := values[p.prefix+"Proxy"], values[p.prefix+"Port"]
		if values[p.prefix+"Enable"] != "1" || host == "" {
			continue
		}
		u, err := parse(net.JoinHostPort(host, port), p.scheme)
		if err != nil {
			return Setting{}, err
		}
		s.URL = u
		break
	}
	if s.URL == nil {
		return Setting{}, nil
	}
	s.Bypass = exceptions
	if values["ExcludeSimpleHostnames"] == "1" {
		s.Bypass = append(s.Bypass, "<local>")
	}
	return s, nil
}

// ParseWindows reads the system proxy from Windows's Internet Options:
// ProxyServer is "host:port" for every protocol, or protocol=host:port
// pairs separated by semicolons, of which the HTTPS, HTTP or SOCKS proxy
// is taken in this order; ProxyOverride lists the bypassed hosts,
// separated by semicolons.
func ParseWindows(server, override string) (Setting, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return Setting{}, nil
	}
	var v, scheme string
	if !strings.Contains(server, "=") {
		v, scheme = server, "http"
	} else {
		byProtocol := map[string]string{}
		for _, pair := range split(server, ";") {
			k, val, _ := strings.Cut(pair, "=")
			byProtocol[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(val)
		}
		// WinINet speaks SOCKS4 to a socks= proxy.
		for _, p := range []struct{ protocol, scheme string }{{"https", "http"}, {"http", "http"}, {"socks", "socks4"}} {
			if byProtocol[p.protocol] != "" {
				v, scheme = byProtocol[p.protocol], p.scheme
				break
			}
		}
		if v == "" {
			return Setting{}, nil
		}
	}
	u, err := parse(v, scheme)
	if err != nil {
		return Setting{}, err
	}
	return Setting{URL: u, Bypass: split(override, ";")}, nil
}
