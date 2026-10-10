package profile

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBuiltinProfiles(t *testing.T) {
	s, err := NewSet(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		d    Device
		want string
	}{
		{"samsung by description", Device{Manufacturer: "Samsung Electronics", ModelName: "UE55TU7100"}, "Samsung TV"},
		{"samsung by header", Device{Header: http.Header{"X-Av-Client-Info": {`av=5.0; cn="Samsung Electronics"; mn="UE55"`}}}, "Samsung TV"},
		{"lg by user agent", Device{Header: http.Header{"User-Agent": {"Linux/3.10 UPnP/1.0 webOS TV"}}}, "LG TV"},
		{"bravia", Device{Manufacturer: "Sony Corporation", ModelName: "KD-55X85J"}, "Sony Bravia"},
		{"kodi", Device{FriendlyName: "Kodi (living room)", ModelName: "Kodi"}, "Kodi"},
		{"vlc client", Device{Header: http.Header{"User-Agent": {"VLC/3.0.20 LibVLC/3.0.20"}}}, "VLC"},
		{"unknown", Device{Manufacturer: "ACME", Header: http.Header{"User-Agent": {"curl"}}}, GenericName},
		// A Samsung phone is not a television.
		{"samsung phone", Device{Manufacturer: "Samsung", ModelName: "Galaxy S24"}, GenericName},
	} {
		if got := s.Match(tt.d).Name; got != tt.want {
			t.Errorf("%s: Match() = %q, want = %q", tt.name, got, tt.want)
		}
	}
	g := s.Generic()
	if g.Name != GenericName || len(g.ClientCapabilities().GetDirectPlay()) == 0 || g.ClientCapabilities().GetName() != "DLNA Generic" {
		t.Errorf("generic = %v", g.ClientCapabilities())
	}
	if p, _ := s.Get("Samsung TV"); p.MimeType("mkv", "video") != "video/x-mkv" || p.MimeType("mov,mp4,m4a,3gp,3g2,mj2", "video") != "video/mp4" {
		t.Errorf("Samsung MIME types = %q, %q", p.MimeType("mkv", "video"), p.MimeType("mov,mp4,m4a,3gp,3g2,mj2", "video"))
	}
}

func TestOverrides(t *testing.T) {
	var overrides []Profile
	err := json.Unmarshal([]byte(`[
		{"name": "Samsung TV", "match": [{"manufacturer": "nobody"}], "capabilities": {"directPlay": [{"kind": "MEDIA_KIND_VIDEO"}]}},
		{"name": "Mine", "match": [{"headers": {"User-Agent": "mine"}}], "time_seek": true}
	]`), &overrides)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSet(overrides)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Match(Device{Manufacturer: "Samsung", ModelName: "TV"}).Name; got != GenericName {
		t.Errorf("replaced Samsung profile matched as %q", got)
	}
	if got := s.Match(Device{Header: http.Header{"User-Agent": {"Mine/1"}}}); got.Name != "Mine" || !got.TimeSeek {
		t.Errorf("Match() = %+v", got)
	}
	for _, bad := range []string{
		`[{"name": ""}]`,
		`[{"name": "x", "match": [{}]}]`,
		`[{"name": "x", "match": [{"model_name": "("}]}]`,
		`[{"name": "x", "capabilities": {"directPlay": [{"kind": "MEDIA_KIND_UNSPECIFIED"}]}}]`,
		`[{"name": "x", "capabilities": {"nonsense": 1}}]`,
		`[{"name": "x"}, {"name": "x"}]`,
	} {
		var o []Profile
		if err := json.Unmarshal([]byte(bad), &o); err != nil {
			t.Fatal(err)
		}
		if _, err := NewSet(o); err == nil {
			t.Errorf("NewSet(%s) = nil error", bad)
		}
	}
}

func TestContainsName(t *testing.T) {
	for _, tt := range []struct {
		list, names string
		want        bool
	}{
		{"", "mkv", true},
		{"mp4,m4v", "mov,mp4,m4a,3gp,3g2,mj2", true},
		{"mp4, mkv", "mkv", true},
		{"ts", "mkv", false},
		{"-mkv", "mkv", false},
		{"-mkv", "mp4", true},
	} {
		if got := ContainsName(tt.list, tt.names); got != tt.want {
			t.Errorf("ContainsName(%q, %q) = %v, want = %v", tt.list, tt.names, got, tt.want)
		}
	}
}
