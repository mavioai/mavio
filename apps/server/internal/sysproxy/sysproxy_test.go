package sysproxy

import (
	"reflect"
	"strings"
	"testing"
)

func urlOf(s Setting) string {
	if s.URL == nil {
		return ""
	}
	return s.URL.String()
}

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		ok      bool
		want    string
		bypass  []string
		wantErr bool
	}{
		{name: "none", env: map[string]string{"NO_PROXY": "localhost"}},
		{
			name: "https first", ok: true, want: "http://127.0.0.1:7890", bypass: []string{"localhost", ".lan"},
			env: map[string]string{"HTTPS_PROXY": "127.0.0.1:7890", "HTTP_PROXY": "http://other:3128", "no_proxy": "localhost, .lan"},
		},
		{name: "lower case", ok: true, want: "http://proxy:80", env: map[string]string{"http_proxy": "http://proxy"}},
		{name: "socks5h", ok: true, want: "socks5://u:p@proxy:1080", env: map[string]string{"ALL_PROXY": "socks5h://u:p@proxy"}},
		{name: "socks4a", ok: true, want: "socks4://proxy:9050", env: map[string]string{"all_proxy": "socks4a://proxy:9050"}},
		{name: "https scheme", ok: true, wantErr: true, env: map[string]string{"HTTPS_PROXY": "https://proxy:443"}},
		{name: "bad port", ok: true, wantErr: true, env: map[string]string{"HTTP_PROXY": "http://proxy:99999"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok, err := FromEnv(func(k string) string { return tt.env[k] })
			if (err != nil) != tt.wantErr || ok != tt.ok {
				t.Fatalf("FromEnv() ok = %v, err = %v; want = %v, error %v", ok, err, tt.ok, tt.wantErr)
			}
			if got := urlOf(s); got != tt.want {
				t.Errorf("URL got = %q, want = %q", got, tt.want)
			}
			if !reflect.DeepEqual(s.Bypass, tt.bypass) {
				t.Errorf("Bypass got = %v, want = %v", s.Bypass, tt.bypass)
			}
		})
	}
}

const scutil = `<dictionary> {
  ExceptionsList : <array> {
    0 : 192.168.0.0/16
    1 : *.local
    2 : 127.0.0.1/32
  }
  ExcludeSimpleHostnames : 1
  FTPPassive : 1
  HTTPEnable : 1
  HTTPPort : 1080
  HTTPProxy : 127.0.0.1
  HTTPSEnable : HTTPS_ENABLE
  HTTPSPort : 8443
  HTTPSProxy : 10.0.0.2
  ProxyAutoConfigEnable : 0
  SOCKSEnable : 0
}
`

func TestParseScutil(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		want   string
		bypass []string
	}{
		{
			name: "https", out: replace(scutil, "1"), want: "http://10.0.0.2:8443",
			bypass: []string{"192.168.0.0/16", "*.local", "127.0.0.1/32", "<local>"},
		},
		{
			name: "http", out: replace(scutil, "0"), want: "http://127.0.0.1:1080",
			bypass: []string{"192.168.0.0/16", "*.local", "127.0.0.1/32", "<local>"},
		},
		{
			name: "socks", want: "socks5://127.0.0.1:1086",
			out: "<dictionary> {\n  SOCKSEnable : 1\n  SOCKSPort : 1086\n  SOCKSProxy : 127.0.0.1\n}\n",
		},
		{name: "none", out: "<dictionary> {\n  HTTPEnable : 0\n  SOCKSEnable : 0\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseScutil([]byte(tt.out))
			if err != nil {
				t.Fatal(err)
			}
			if got := urlOf(s); got != tt.want {
				t.Errorf("URL got = %q, want = %q", got, tt.want)
			}
			if !reflect.DeepEqual(s.Bypass, tt.bypass) {
				t.Errorf("Bypass got = %v, want = %v", s.Bypass, tt.bypass)
			}
		})
	}
}

func replace(out, httpsEnable string) string {
	return strings.Replace(out, "HTTPS_ENABLE", httpsEnable, 1)
}

func TestParseWindows(t *testing.T) {
	tests := []struct {
		server, override string
		want             string
		bypass           []string
	}{
		{server: "127.0.0.1:7890", override: "localhost;127.*;<local>", want: "http://127.0.0.1:7890", bypass: []string{"localhost", "127.*", "<local>"}},
		{server: "http=proxy:3128;https=secure:8443", want: "http://secure:8443"},
		{server: "ftp=proxy:21;socks=proxy:1080", want: "socks4://proxy:1080"},
		{server: "ftp=proxy:21"},
		{server: ""},
	}
	for _, tt := range tests {
		s, err := ParseWindows(tt.server, tt.override)
		if err != nil {
			t.Fatalf("ParseWindows(%q): %v", tt.server, err)
		}
		if got := urlOf(s); got != tt.want {
			t.Errorf("ParseWindows(%q) URL got = %q, want = %q", tt.server, got, tt.want)
		}
		if !reflect.DeepEqual(s.Bypass, tt.bypass) {
			t.Errorf("ParseWindows(%q) Bypass got = %v, want = %v", tt.server, s.Bypass, tt.bypass)
		}
	}
}
