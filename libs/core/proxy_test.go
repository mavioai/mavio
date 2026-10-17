package core

import (
	"errors"
	"testing"
)

func TestProxyValidate(t *testing.T) {
	id := NewID()
	for _, tc := range []struct {
		name string
		p    Proxy
		ok   bool
	}{
		{"http", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "proxy.example.com", Port: 8080}, true},
		{"socks5 with credentials", Proxy{ID: id, Name: "a", Kind: ProxySOCKS5, Host: "10.0.0.1", Port: 1080, Username: "u", Password: "p"}, true},
		{"IPv6", Proxy{ID: id, Name: "a", Kind: ProxySOCKS5, Host: "::1", Port: 1080}, true},
		{"socks4 with a user ID", Proxy{ID: id, Name: "a", Kind: ProxySOCKS4, Host: "h", Port: 1080, Username: "u"}, true},
		{"no ID", Proxy{Name: "a", Kind: ProxyHTTP, Host: "h", Port: 1}, false},
		{"no name", Proxy{ID: id, Name: " ", Kind: ProxyHTTP, Host: "h", Port: 1}, false},
		{"unknown kind", Proxy{ID: id, Name: "a", Kind: "https", Host: "h", Port: 1}, false},
		{"bracketed IPv6", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "[::1]", Port: 1}, false},
		{"host with a port", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "h:80", Port: 1}, false},
		{"port 0", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "h", Port: 0}, false},
		{"port 65536", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "h", Port: 65536}, false},
		{"socks4 with a password", Proxy{ID: id, Name: "a", Kind: ProxySOCKS4, Host: "h", Port: 1, Username: "u", Password: "p"}, false},
		{"password without username", Proxy{ID: id, Name: "a", Kind: ProxyHTTP, Host: "h", Port: 1, Password: "p"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.ok && err != nil || !tc.ok && !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate() = %v, want ok = %v", err, tc.ok)
			}
		})
	}
}

func TestProxyKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b Proxy
		same bool
	}{
		{"kind ignored", Proxy{Kind: ProxyHTTP, Host: "h", Port: 1}, Proxy{Kind: ProxySOCKS5, Host: "h", Port: 1}, true},
		{"host case", Proxy{Host: "Proxy.Example.com", Port: 1}, Proxy{Host: "proxy.example.com.", Port: 1}, true},
		{"IPv6 forms", Proxy{Host: "2001:db8:0:0::1", Port: 1}, Proxy{Host: "2001:DB8::1", Port: 1}, true},
		{"IPv4-mapped", Proxy{Host: "::ffff:10.0.0.1", Port: 1}, Proxy{Host: "10.0.0.1", Port: 1}, true},
		{"password ignored", Proxy{Host: "h", Port: 1, Username: "u", Password: "a"}, Proxy{Host: "h", Port: 1, Username: "u", Password: "b"}, true},
		{"other port", Proxy{Host: "h", Port: 1}, Proxy{Host: "h", Port: 2}, false},
		{"other username", Proxy{Host: "h", Port: 1, Username: "a"}, Proxy{Host: "h", Port: 1, Username: "b"}, false},
		{"name not resolved", Proxy{Host: "localhost", Port: 1}, Proxy{Host: "127.0.0.1", Port: 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Key() == tc.b.Key(); got != tc.same {
				t.Errorf("Key() %q == %q: got = %v, want = %v", tc.a.Key(), tc.b.Key(), got, tc.same)
			}
		})
	}
}

func TestProxyURL(t *testing.T) {
	for _, tc := range []struct {
		p    Proxy
		want string
	}{
		{Proxy{Kind: ProxyHTTP, Host: "h", Port: 8080}, "http://h:8080"},
		{Proxy{Kind: ProxySOCKS5, Host: "::1", Port: 1080, Username: "u", Password: "p@ss"}, "socks5://u:p%40ss@[::1]:1080"},
		{Proxy{Kind: ProxySOCKS4, Host: "h", Port: 1080, Username: "u"}, "socks4://u@h:1080"},
	} {
		if got := tc.p.URL().String(); got != tc.want {
			t.Errorf("URL() = %q, want = %q", got, tc.want)
		}
	}
}
