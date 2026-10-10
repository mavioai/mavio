package main

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/plugins/dlna/internal/didl"
	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
)

func TestManifest(t *testing.T) {
	m, err := manifest.Load(".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.GetId() != ID || m.GetVersion() != Version {
		t.Errorf("manifest id, version = %s %s, want = %s %s", m.GetId(), m.GetVersion(), ID, Version)
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal([]byte(m.GetConfigSchema()), &schema); err != nil {
		t.Fatalf("config schema: %v", err)
	}
	// Every field of the configuration is in the schema, which allows no
	// others.
	var c config
	data, _ := json.Marshal(c)
	var fields map[string]any
	_ = json.Unmarshal(data, &fields)
	for f := range fields {
		if _, ok := schema.Properties[f]; !ok {
			t.Errorf("config field %q is not in the schema", f)
		}
	}
	if len(fields) != len(schema.Properties) {
		t.Errorf("config fields = %d, schema properties = %d", len(fields), len(schema.Properties))
	}
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		in    string
		ok    bool
		kinds []libraryv1.ItemKind
		text  string
	}{
		{in: "*", ok: true},
		{in: "", ok: true},
		{in: `upnp:class derivedfrom "object.item.audioItem"`, ok: true, kinds: []libraryv1.ItemKind{
			libraryv1.ItemKind_ITEM_KIND_TRACK, libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK,
		}},
		{in: `upnp:class = "object.container.album.musicAlbum"`, ok: true, kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM}},
		{
			in: `(upnp:class derivedfrom "object.item.videoItem" or upnp:class = "object.item.imageItem.photo") and dc:title contains "Wire \"x\""`, ok: true,
			kinds: []libraryv1.ItemKind{
				libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_EPISODE, libraryv1.ItemKind_ITEM_KIND_VIDEO,
				libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, libraryv1.ItemKind_ITEM_KIND_PHOTO,
			}, text: `Wire "x"`,
		},
		{in: `upnp:class derivedfrom "object.item.textItem"`, ok: true, kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_UNSPECIFIED}},
		{in: `dc:title contains "amé"`, ok: true, text: "amé"},
		{in: `upnp:genre = "Jazz"`},
	}
	for _, tt := range tests {
		q, ok := parseSearch(tt.in)
		want := slices.Clone(tt.kinds)
		slices.Sort(want)
		if ok != tt.ok || !slices.Equal(q.kinds, want) || q.text != tt.text {
			t.Errorf("parseSearch(%q) got = %v %q %v, want = %v %q %v", tt.in, q.kinds, q.text, ok, want, tt.text, tt.ok)
		}
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		in   string
		ok   bool
		want []string
	}{
		{in: "", ok: true},
		{in: "+dc:title", ok: true, want: []string{"SORT_FIELD_NAME"}},
		{in: "-dc:date, +upnp:originalTrackNumber,+upnp:genre", ok: true, want: []string{"-SORT_FIELD_PREMIERE_DATE", "SORT_FIELD_INDEX"}},
		{in: "dc:title"},
	}
	for _, tt := range tests {
		specs, ok := parseSort(tt.in)
		var got []string
		for _, s := range specs {
			name := s.GetField().String()
			if s.GetDescending() {
				name = "-" + name
			}
			got = append(got, name)
		}
		if ok != tt.ok || !slices.Equal(got, tt.want) {
			t.Errorf("parseSort(%q) got = %v %v, want = %v %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func source(container string, streams ...*libraryv1.MediaStream) *libraryv1.MediaSource {
	return libraryv1.MediaSource_builder{Container: proto.String(container), Streams: streams}.Build()
}

func mstream(kind libraryv1.StreamKind, codec string) *libraryv1.MediaStream {
	return libraryv1.MediaStream_builder{Kind: kind.Enum(), Codec: proto.String(codec)}.Build()
}

func TestFormatFor(t *testing.T) {
	set, err := profile.NewSet(nil)
	if err != nil {
		t.Fatal(err)
	}
	generic := set.Generic()
	samsung, _ := set.Get("Samsung TV")
	video, audio := libraryv1.StreamKind_STREAM_KIND_VIDEO, libraryv1.StreamKind_STREAM_KIND_AUDIO
	tests := []struct {
		name string
		p    *profile.Profile
		kind string
		ms   *libraryv1.MediaSource
		want streamFormat
	}{
		{
			"mp4 direct", generic, "video", source("mov,mp4,m4a,3gp,3g2,mj2", mstream(video, "h264"), mstream(audio, "aac")),
			streamFormat{direct: true, container: "mov,mp4,m4a,3gp,3g2,mj2", mime: "video/mp4"},
		},
		{
			"mkv transcoded", generic, "video", source("mkv", mstream(video, "hevc"), mstream(audio, "dts")),
			streamFormat{container: "ts", mime: "video/mp2t"},
		},
		{"flac direct", generic, "audio", source("flac", mstream(audio, "flac")), streamFormat{direct: true, container: "flac", mime: "audio/flac"}},
		{"opus transcoded", generic, "audio", source("ogg", mstream(audio, "opus")), streamFormat{container: "mp3", mime: "audio/mpeg"}},
		{
			"samsung mkv", samsung, "video", source("mkv", mstream(video, "h264"), mstream(audio, "aac")),
			streamFormat{direct: true, container: "mkv", mime: "video/x-mkv"},
		},
	}
	for _, tt := range tests {
		if got := formatFor(tt.p, tt.kind, tt.ms); got != tt.want {
			t.Errorf("%s: formatFor() got = %+v, want = %+v", tt.name, got, tt.want)
		}
	}
}

func TestContentFeatures(t *testing.T) {
	tests := []struct {
		direct, timeSeek bool
		want             string
	}{
		{true, false, "DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=" + streamingFlags},
		{false, true, "DLNA.ORG_OP=10;DLNA.ORG_CI=1;DLNA.ORG_FLAGS=" + streamingFlags},
		{false, false, "DLNA.ORG_OP=00;DLNA.ORG_CI=1;DLNA.ORG_FLAGS=" + streamingFlags},
	}
	for _, tt := range tests {
		if got := contentFeatures(tt.direct, tt.timeSeek); got != tt.want {
			t.Errorf("contentFeatures(%v, %v) got = %s, want = %s", tt.direct, tt.timeSeek, got, tt.want)
		}
	}
}

func TestParseTimeSeek(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"npt=10.5-", 10500 * time.Millisecond, true},
		{"npt=0:01:02.500-0:02:00", 62500 * time.Millisecond, true},
		{" npt=0-", 0, true},
		{"", 0, false},
		{"bytes=0-", 0, false},
		{"npt=-5-", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseTimeSeek(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseTimeSeek(%q) got = %v %v, want = %v %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
	if got := npt(90*time.Second + 250*time.Millisecond); got != "90.250" {
		t.Errorf("npt() got = %s, want = 90.250", got)
	}
}

func TestSmallHelpers(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"max-age=1800": 1800 * time.Second, "no-cache, max-age = 66": 66 * time.Second, "": defaultMaxAge, "max-age=x": defaultMaxAge,
	} {
		if got := maxAge(in); got != want {
			t.Errorf("maxAge(%q) got = %v, want = %v", in, got, want)
		}
	}
	for in, want := range map[time.Duration]string{0: "0:00:00", 3723*time.Second + 900*time.Millisecond: "1:02:03"} {
		if got := clock(in); got != want {
			t.Errorf("clock(%v) got = %s, want = %s", in, got, want)
		}
	}
	for in, want := range map[string]string{"mov,mp4,m4a,3gp,3g2,mj2": "mp4", "ts": "ts", "": "bin"} {
		if got := extension(in); got != want {
			t.Errorf("extension(%q) got = %s, want = %s", in, got, want)
		}
	}
	if got := truncate("ééé", 5); got != "éé" {
		t.Errorf("truncate() got = %q, want = éé", got)
	}
	if d, err := didl.ParseDuration(clock(time.Hour)); err != nil || d != time.Hour {
		t.Errorf("ParseDuration(clock(1h)) got = %v, %v", d, err)
	}
}
