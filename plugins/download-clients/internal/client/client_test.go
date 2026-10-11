package client

import (
	"context"
	"crypto/sha1" //nolint:gosec // BitTorrent v1 info hashes are SHA-1.
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMagnetHash(t *testing.T) {
	tests := []struct {
		link, want string
		ok         bool
	}{
		// Radarr's TransmissionFixture.Download_should_get_hash_from_magnet_url.
		{"magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp", "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", true},
		{"magnet:?xt=urn:btih:cbc2f069fe8bb2f544eae707d75bcd3de9dcf951&dn=x", "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", true},
		{"magnet:?xt=urn:btmh:1220" + strings.Repeat("ab", 32), strings.Repeat("AB", 32), true},
		{
			"magnet:?xt=urn:btmh:1220" + strings.Repeat("ab", 32) + "&xt=urn:btih:cbc2f069fe8bb2f544eae707d75bcd3de9dcf951",
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", true,
		},
		{"magnet:?dn=nohash", "", false},
		{"http://example.com/a.torrent", "", false},
	}
	for _, tt := range tests {
		got, err := MagnetHash(tt.link)
		if got != tt.want || (err == nil) != tt.ok {
			t.Errorf("MagnetHash(%q) = %q, %v, want = %q, ok %v", tt.link, got, err, tt.want, tt.ok)
		}
	}
}

func TestInfoHash(t *testing.T) {
	info := "d6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaae"
	v1 := sha1.Sum([]byte(info)) //nolint:gosec // BitTorrent v1 info hashes are SHA-1.
	v2info := "d9:file treede12:meta versioni2e4:name1:a12:piece lengthi16384ee"
	v2 := sha256.Sum256([]byte(v2info))
	tests := []struct {
		name, data, want string
		ok               bool
	}{
		{"v1", "d8:announce3:url4:info" + info + "e", strings.ToUpper(hex.EncodeToString(v1[:])), true},
		{"v2 only", "d4:info" + v2info + "e", strings.ToUpper(hex.EncodeToString(v2[:])), true},
		{"no info", "d8:announce3:urle", "", false},
		{"truncated", "d4:infod6:lengthi1e", "", false},
		{"not bencoded", "<html>", "", false},
	}
	for _, tt := range tests {
		got, err := InfoHash([]byte(tt.data))
		if got != tt.want || (err == nil) != tt.ok {
			t.Errorf("%s: InfoHash() = %q, %v, want = %q, ok %v", tt.name, got, err, tt.want, tt.ok)
		}
	}
}

// adder records what AddTorrent sends.
type adder struct{ magnet, file string }

func (a *adder) AddMagnet(_ context.Context, _ *Release, hash, link string) (string, error) {
	a.magnet = link
	return "", nil
}

func (a *adder) AddTorrentFile(_ context.Context, _ *Release, hash, name string, _ []byte) (string, error) {
	a.file = name
	return strings.ToLower(hash), nil
}

func TestAddTorrent(t *testing.T) {
	data := []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	hash, _ := InfoHash(data)
	link := "magnet:?xt=urn:btih:cbc2f069fe8bb2f544eae707d75bcd3de9dcf951"
	tests := []struct {
		name       string
		r          Release
		want, file string
		magnet     string
		ok         bool
	}{
		{"file before magnet", Release{Title: "A: B?", Data: data, MagnetURL: link}, hash, "A - B!.torrent", "", true},
		{"file name kept", Release{Title: "A", Data: data, FileName: "x.torrent"}, hash, "x.torrent", "", true},
		{"magnet", Release{Title: "A", MagnetURL: link}, "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", "", link, true},
		{"neither", Release{Title: "A"}, "", "", "", false},
	}
	for _, tt := range tests {
		a := &adder{}
		got, err := AddTorrent(context.Background(), nil, a, &tt.r)
		if got != tt.want || a.file != tt.file || a.magnet != tt.magnet || (err == nil) != tt.ok {
			t.Errorf("%s: AddTorrent() = %q, %v with file %q, magnet %q, want = %q with %q, %q",
				tt.name, got, err, a.file, a.magnet, tt.want, tt.file, tt.magnet)
		}
	}
}

func TestCleanFileName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Mission: Impossible", "Mission - Impossible"},
		{"10:30", "10-30"},
		{`AC/DC\Live`, "AC+DC+Live"},
		{"Why?", "Why!"},
		{"M*A*S*H", "M-A-S-H"},
		{`a<b>c|d"e`, "abcde"},
		{" .hidden ", "hidden"},
	}
	for _, tt := range tests {
		if got := CleanFileName(tt.in); got != tt.want {
			t.Errorf("CleanFileName(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

func TestServer(t *testing.T) {
	tests := []struct {
		s    Server
		want string
	}{
		{Server{Host: "localhost", Port: 9091, URLBase: "/transmission/"}, "http://localhost:9091/transmission"},
		{Server{Host: "nas", Port: 443, UseSSL: true, URLBase: "qbt"}, "https://nas:443/qbt"},
		{Server{Host: "::1", Port: 8080}, "http://[::1]:8080"},
	}
	for _, tt := range tests {
		if got := tt.s.BaseURL(); got != tt.want {
			t.Errorf("BaseURL() = %q, want = %q", got, tt.want)
		}
	}
	if err := (Server{Host: "h", Port: 0}).Validate(); err == nil {
		t.Errorf("Validate() of port 0 = nil, want an error")
	}
}

func TestCategories(t *testing.T) {
	c := Categories{MovieCategory: "movies", TVCategory: "tv", TVImportedCategory: "tv-done"}
	if c.Category(Series) != "tv" || c.Category(Movie) != "movies" || c.Category(0) != "movies" {
		t.Errorf("Category() = %q, %q, want tv, movies", c.Category(Series), c.Category(Movie))
	}
	if !c.HasPostImportCategory(Series) || c.HasPostImportCategory(Movie) {
		t.Errorf("HasPostImportCategory() = %v, %v, want true, false", c.HasPostImportCategory(Series), c.HasPostImportCategory(Movie))
	}
	if k, ok := c.Match(func(s string) bool { return s == "tv" }); k != Series || !ok {
		t.Errorf("Match(tv) = %v, %v, want = series", k, ok)
	}
	if _, ok := c.Match(func(string) bool { return false }); ok {
		t.Errorf("Match(none) = true, want false")
	}
	p := Priorities{RecentMoviePriority: 1, OlderMoviePriority: 2, RecentTVPriority: 3, OlderTVPriority: 4}
	if p.Priority(Movie, true) != 1 || p.Priority(Movie, false) != 2 || p.Priority(Series, true) != 3 || p.Priority(Series, false) != 4 {
		t.Errorf("Priority() = %+v, want each by kind and recency", p)
	}
}

func TestStatuses(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	file := filepath.Join(t.TempDir(), "status.json")
	s, err := NewStatuses(file, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	s.SetStarted(now.Add(-time.Hour))
	s.RecordFailure("a", "down")
	until, failure, ok := s.Disabled("a")
	if !ok || failure != "down" || !until.Equal(now.Add(backoffPeriods[1])) {
		t.Errorf("Disabled() = %v, %q, %v, want disabled for %v", until, failure, ok, backoffPeriods[1])
	}
	s.RecordFailure("a", "down")
	if got := s.Get("a").EscalationLevel; got != 2 {
		t.Errorf("EscalationLevel = %d, want = 2", got)
	}
	reloaded, err := NewStatuses(file, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get("a").EscalationLevel; got != 2 {
		t.Errorf("reloaded EscalationLevel = %d, want = 2", got)
	}
	s.RecordSuccess("a")
	if _, _, ok := s.Disabled("a"); ok || s.Get("a").EscalationLevel != 1 {
		t.Errorf("after success: disabled %v, level %d, want enabled at 1", ok, s.Get("a").EscalationLevel)
	}
	s.Remove(func(string) bool { return false })
	if s.Get("a") != (ClientStatus{}) {
		t.Errorf("Get() after Remove = %+v, want zero", s.Get("a"))
	}
}

func TestResponseErr(t *testing.T) {
	var auth *AuthError
	if err := (&Response{StatusCode: 401}).Err(); !errors.As(err, &auth) {
		t.Errorf("Err() of 401 = %v, want an AuthError", err)
	}
	var h *HTTPError
	if err := (&Response{StatusCode: 500, Body: []byte("oops")}).Err(); !errors.As(err, &h) || h.Body != "oops" {
		t.Errorf("Err() of 500 = %v, want an HTTPError", err)
	}
	if err := (&Response{StatusCode: 204}).Err(); err != nil {
		t.Errorf("Err() of 204 = %v, want nil", err)
	}
}
