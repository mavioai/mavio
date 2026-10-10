package didl

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestMarshal(t *testing.T) {
	got := Marshal([]Object{
		{ID: "lib", ParentID: "0", Container: true, ChildCount: 2, Searchable: true, Title: "Films & Shows", Class: ClassStorageFolder},
		{
			ID: "m1", ParentID: "lib", Title: "Up", Class: ClassMovie, Date: time.Date(2009, 5, 29, 0, 0, 0, 0, time.UTC),
			Genres: []string{"Animation"}, AlbumArt: "http://h/art?a=1&b=2", AlbumArtProfile: "JPEG_TN",
			Res:     []Res{{URL: "http://h/media/m1?p=x&q=y", ProtocolInfo: "http-get:*:video/mp4:DLNA.ORG_OP=01", Size: 100, Duration: 96*time.Minute + 1500*time.Millisecond, Resolution: "1920x1080"}},
			Caption: "http://h/sub.srt", CaptionType: "srt",
		},
		{ID: "a", ParentID: "lib", Container: true, ChildCount: -1, Title: "Album", Class: ClassMusicAlbum},
	})
	for _, want := range []string{
		`<container id="lib" parentID="0" restricted="1" childCount="2" searchable="1"><dc:title>Films &amp; Shows</dc:title>`,
		`<dc:date>2009-05-29</dc:date>`,
		`<upnp:albumArtURI dlna:profileID="JPEG_TN">http://h/art?a=1&amp;b=2</upnp:albumArtURI>`,
		`<res protocolInfo="http-get:*:video/mp4:DLNA.ORG_OP=01" size="100" duration="1:36:01.500" resolution="1920x1080">http://h/media/m1?p=x&amp;q=y</res>`,
		`<sec:CaptionInfoEx sec:type="srt">http://h/sub.srt</sec:CaptionInfoEx>`,
		`<container id="a" parentID="lib" restricted="1" searchable="0">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Marshal() lacks %s\n%s", want, got)
		}
	}
	// It is well-formed XML.
	d := xml.NewDecoder(strings.NewReader(got))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() != "EOF" {
				t.Errorf("not well-formed: %v", err)
			}
			break
		}
	}
}

func TestParseDuration(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"0:00:05", 5 * time.Second, true},
		{"1:36:01.500", 96*time.Minute + 1500*time.Millisecond, true},
		{"01:02:03.25", time.Hour + 2*time.Minute + 3250*time.Millisecond, true},
		{"0:00:01.1/2", 1500 * time.Millisecond, true},
		{"NOT_IMPLEMENTED", 0, false},
		{"0:61:00", 0, false},
		{"", 0, false},
	} {
		got, err := ParseDuration(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, %v, want = %v", tt.in, got, err, tt.want)
		}
	}
	if got := Duration(time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond); got != "1:02:03.004" {
		t.Errorf("Duration() = %q", got)
	}
}
