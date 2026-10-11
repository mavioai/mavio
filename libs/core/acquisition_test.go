package core

import (
	"errors"
	"testing"
	"time"
)

// validator is a domain value with invariants.
type validator interface{ Validate() error }

func TestAcquisitionValidate(t *testing.T) {
	id := NewID()
	profile := func(change func(*QualityProfile)) validator {
		p := QualityProfile{ID: id, Kind: KindMovie, Name: "HD", Cutoff: 7, Items: []QualityProfileItem{{Quality: 7, Allowed: true}}}
		change(&p)
		return &p
	}
	delay := func(change func(*DelayProfile)) validator {
		p := DelayProfile{ID: id, Default: true, EnableUsenet: true, EnableTorrent: true, PreferredProtocol: ProtocolUsenet}
		change(&p)
		return &p
	}
	settings := func(change func(*AcquisitionSettings)) validator {
		s := DefaultAcquisitionSettings()
		change(&s)
		return &s
	}
	tests := []struct {
		name string
		v    validator
		ok   bool
	}{
		{"definition", &QualityDefinition{Kind: KindMovie, Quality: 7, Title: "Bluray-1080p", MinSize: 2, MaxSize: 100, PreferredSize: 95}, true},
		{"definition of an episode", &QualityDefinition{Kind: KindEpisode, Quality: 7, Title: "Bluray-1080p"}, false},
		{"definition above maximum", &QualityDefinition{Kind: KindMovie, Quality: 7, Title: "Bluray-1080p", MinSize: 10, MaxSize: 5}, false},
		{"definition preferring outside", &QualityDefinition{Kind: KindSeries, Quality: 7, Title: "Bluray-1080p", MaxSize: 5, PreferredSize: 6}, false},
		{"profile", profile(func(*QualityProfile) {}), true},
		{"profile without a name", profile(func(p *QualityProfile) { p.Name = " " }), false},
		{"profile without items", profile(func(p *QualityProfile) { p.Items = nil }), false},
		{"profile scoring twice", profile(func(p *QualityProfile) {
			f := NewID()
			p.FormatScores = []FormatScore{{FormatID: f, Score: 1}, {FormatID: f, Score: 2}}
		}), false},
		{"custom format", &CustomFormat{ID: id, Name: "x265", Specifications: []CustomFormatSpec{{Kind: SpecReleaseTitle, Name: "x265", Value: `x265`}}}, true},
		{"custom format of unknown kind", &CustomFormat{ID: id, Name: "x265", Specifications: []CustomFormatSpec{{Kind: "codec", Name: "x265"}}}, false},
		{"default delay profile", delay(func(*DelayProfile) {}), true},
		{"default delay profile with tags", delay(func(p *DelayProfile) { p.Tags = []string{"anime"} }), false},
		{"delay profile without tags", delay(func(p *DelayProfile) { p.Default = false }), false},
		{"delay profile without protocols", delay(func(p *DelayProfile) { p.EnableUsenet, p.EnableTorrent = false, false }), false},
		{"delay profile with a negative delay", delay(func(p *DelayProfile) { p.UsenetDelay = -time.Minute }), false},
		{"release profile", &ReleaseProfile{ID: id, Required: []string{"/WEB\\b/i"}}, true},
		{"release profile without terms", &ReleaseProfile{ID: id}, false},
		{"release profile with an empty term", &ReleaseProfile{ID: id, Ignored: []string{" "}}, false},
		{"wanted", &Wanted{ItemID: id, QualityProfileID: id, MinimumAvailability: AvailabilityReleased}, true},
		{"wanted without a profile", &Wanted{ItemID: id}, false},
		{"wanted with unknown availability", &Wanted{ItemID: id, QualityProfileID: id, MinimumAvailability: "tba"}, false},
		{"download", &Download{ID: id, ItemID: id, PluginID: "p", ClientID: "c", DownloadID: "d", Protocol: ProtocolTorrent, State: DownloadDownloading}, true},
		{"download without a client", &Download{ID: id, ItemID: id, Protocol: ProtocolTorrent, State: DownloadDownloading}, false},
		{"download of unknown state", &Download{ID: id, ItemID: id, PluginID: "p", ClientID: "c", DownloadID: "d", Protocol: ProtocolTorrent, State: "paused"}, false},
		{"history event", &HistoryEvent{ID: id, ItemID: id, Type: HistoryGrabbed, Date: time.Now()}, true},
		{"history event of unknown type", &HistoryEvent{ID: id, ItemID: id, Type: "searched", Date: time.Now()}, false},
		{"blocklist entry", &BlocklistEntry{ID: id, ItemID: id, SourceTitle: "Movie.2024", Date: time.Now(), Protocol: ProtocolUsenet}, true},
		{"blocklist entry without a title", &BlocklistEntry{ID: id, ItemID: id, Date: time.Now(), Protocol: ProtocolUsenet}, false},
		{"pending release", &PendingRelease{ID: id, ItemID: id, Added: time.Now(), Reason: PendingDelay, Release: []byte(`{}`)}, true},
		{"pending release without a release", &PendingRelease{ID: id, ItemID: id, Added: time.Now(), Reason: PendingDelay}, false},
		{"remote path mapping", &RemotePathMapping{ID: id, Host: "nas", RemotePath: "/downloads/", LocalPath: `D:\downloads`}, true},
		{"remote path mapping to a relative path", &RemotePathMapping{ID: id, Host: "nas", RemotePath: "/downloads/", LocalPath: "downloads"}, false},
		{"acquisition settings", settings(func(*AcquisitionSettings) {}), true},
		{"RSS sync disabled", settings(func(s *AcquisitionSettings) { s.RSSSyncInterval = 0 }), true},
		{"RSS sync too often", settings(func(s *AcquisitionSettings) { s.RSSSyncInterval = 5 }), false},
		{"relative recycle bin", settings(func(s *AcquisitionSettings) { s.RecycleBin = "trash" }), false},
		{"unknown propers setting", settings(func(s *AcquisitionSettings) { s.Propers = "always" }), false},
		{"no movie file format", settings(func(s *AcquisitionSettings) { s.Naming.StandardMovieFormat = "" }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.v.Validate()
			if (err == nil) != tt.ok || err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate() = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

func TestDownloadStateFinished(t *testing.T) {
	for _, s := range DownloadStates {
		want := s == DownloadImported || s == DownloadFailed || s == DownloadIgnored
		if got := s.Finished(); got != want {
			t.Errorf("%s.Finished() = %v, want = %v", s, got, want)
		}
	}
}
