package client

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestJoinRemote(t *testing.T) {
	tests := []struct {
		dir, name, want string
	}{
		{"", "a", "a"},
		{"/downloads", "a", "/downloads/a"},
		{"/downloads/", "a", "/downloads/a"},
		{"/downloads", "/a", "/downloads/a"},
		{`C:\downloads`, "a", `C:\downloads\a`},
		{`C:\downloads\`, "a", `C:\downloads\a`},
		{"C:", "a", `C:\a`},
		{`\\server\share`, "a", `\\server\share\a`},
		{"C:/downloads", "a", "C:/downloads/a"},
	}
	for _, tt := range tests {
		if got := JoinRemote(tt.dir, tt.name); got != tt.want {
			t.Errorf("JoinRemote(%q, %q) = %q, want = %q", tt.dir, tt.name, got, tt.want)
		}
	}
}

func TestRemoteBase(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/downloads/a", "a"},
		{"/downloads/a/", "a"},
		{`C:\downloads\a`, "a"},
		{"a", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := RemoteBase(tt.path); got != tt.want {
			t.Errorf("RemoteBase(%q) = %q, want = %q", tt.path, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		s    string
		want string
		ok   bool
	}{
		{"1.2", "1.2", true},
		{"0.9.8", "0.9.8", true},
		{"1.2.3.4", "1.2.3.4", true},
		{" 2.84 ", "2.84", true},
		{"1. 2", "1.2", true},
		{"1", "", false},
		{"1.2.3.4.5", "", false},
		{"1.-2", "", false},
		{"1.x", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		v, err := ParseVersion(tt.s)
		if (err == nil) != tt.ok || (tt.ok && v.String() != tt.want) {
			t.Errorf("ParseVersion(%q) = %v, %v, want = %q, ok = %v", tt.s, v, err, tt.want, tt.ok)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		v       Version
		minimum []int
		want    bool
	}{
		{Version{2, 0}, []int{2, 0}, true},
		{Version{2, 0, 1}, []int{2, 0}, true},
		// As .NET's Version, a missing number is below any.
		{Version{2, 0}, []int{2, 0, 0}, false},
		{Version{1, 34, 0}, []int{1, 34, 0}, true},
		{Version{1, 9}, []int{1, 34, 0}, false},
	}
	for _, tt := range tests {
		if got := tt.v.AtLeast(tt.minimum...); got != tt.want {
			t.Errorf("%v.AtLeast(%v) = %v, want = %v", tt.v, tt.minimum, got, tt.want)
		}
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LocalHost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"[::1]", true},
		{"192.168.1.2", false},
		{"localhost.example.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsLocalhost(tt.host); got != tt.want {
			t.Errorf("IsLocalhost(%q) = %v, want = %v", tt.host, got, tt.want)
		}
	}
}

func TestValidateURLBase(t *testing.T) {
	tests := []struct {
		urlBase string
		ok      bool
	}{
		{"", true},
		{"/sabnzbd", true},
		{"sabnzbd", true},
		{"/rpc/http", true},
		{"http://host", false},
		{"/https://host/x", false},
		{"HTTP://host", false},
	}
	for _, tt := range tests {
		if err := ValidateURLBase("url base", tt.urlBase, "/sabnzbd"); (err == nil) != tt.ok {
			t.Errorf("ValidateURLBase(%q) error = %v, want ok = %v", tt.urlBase, err, tt.ok)
		}
	}
}

func TestErrors(t *testing.T) {
	if err := Rejected("NZBGet rejected the NZB for an unknown reason"); !errors.Is(err, ErrReleaseRejected) {
		t.Errorf("Rejected() = %v, want ErrReleaseRejected", err)
	}
	var w *TestWarning
	if err := Warn("develop version"); !errors.As(err, &w) || w.Message != "develop version" {
		t.Errorf("Warn() = %v, want a TestWarning", err)
	}
	if got := (Options{}).Limit(); got != DefaultHistoryLimit {
		t.Errorf("Limit() = %d, want = %d", got, DefaultHistoryLimit)
	}
	if got := (Options{HistoryLimit: 5}).Limit(); got != 5 {
		t.Errorf("Limit() = %d, want = 5", got)
	}
}

func TestSeeds(t *testing.T) {
	file := filepath.Join(t.TempDir(), "seeds.json")
	s, err := NewSeeds(file)
	if err != nil {
		t.Fatal(err)
	}
	ratio, hour := 1.5, time.Hour
	a := s.For("a")
	a.Remember("abc", &Release{SeedRatio: &ratio})
	a.Remember("def", &Release{SeedTime: &hour})
	a.Remember("none", &Release{})
	s.For("b").Remember("abc", &Release{SeedTime: &hour})

	s, err = NewSeeds(file)
	if err != nil {
		t.Fatal(err)
	}
	a = s.For("a")
	if seed, ok := a.Get("ABC"); !ok || *seed.Ratio != ratio || seed.Time != nil {
		t.Errorf("Get(ABC) = %+v, %v, want the ratio", seed, ok)
	}
	if seed, ok := a.Get("def"); !ok || *seed.Time != hour {
		t.Errorf("Get(def) = %+v, %v, want the time", seed, ok)
	}
	if _, ok := a.Get("none"); ok {
		t.Errorf("Get(none) found a release without seed limits")
	}
	a.Forget(func(hash string) bool { return hash == "DEF" })
	s.Remove(func(id string) bool { return id == "a" })

	s, err = NewSeeds(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.For("a").Get("abc"); ok {
		t.Errorf("Get(abc) found a forgotten torrent")
	}
	if _, ok := s.For("a").Get("def"); !ok {
		t.Errorf("Get(def) lost a torrent kept")
	}
	if _, ok := s.For("b").Get("abc"); ok {
		t.Errorf("Get(abc) of b found a removed client's torrent")
	}
}
