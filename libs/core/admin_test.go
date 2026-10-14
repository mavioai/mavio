package core

import (
	"errors"
	"testing"
)

func TestServerSettingsValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ServerSettings)
		ok     bool
	}{
		{"defaults", func(*ServerSettings) {}, true},
		{"base URL", func(s *ServerSettings) { s.Network.BaseURL = "/mavio" }, true},
		{"nested base URL", func(s *ServerSettings) { s.Network.BaseURL = "/apps/mavio" }, true},
		{"base URL with a slash", func(s *ServerSettings) { s.Network.BaseURL = "/mavio/" }, false},
		{"relative base URL", func(s *ServerSettings) { s.Network.BaseURL = "mavio" }, false},
		{"base URL with a query", func(s *ServerSettings) { s.Network.BaseURL = "/mavio?x" }, false},
		{"root base URL", func(s *ServerSettings) { s.Network.BaseURL = "/" }, false},
		{"HTTPS without a certificate", func(s *ServerSettings) { s.Network.HTTPSPort = 8920 }, false},
		{"HTTPS", func(s *ServerSettings) {
			s.Network.HTTPSPort, s.Network.CertificatePath, s.Network.KeyPath = 8920, "/c.pem", "/k.pem"
		}, true},
		{"unknown acceleration", func(s *ServerSettings) { s.Transcoding.HardwareAcceleration = "qsv" }, false},
		{"unknown preset", func(s *ServerSettings) { s.Transcoding.EncoderPreset = "warp" }, false},
		{"CRF", func(s *ServerSettings) { s.Transcoding.H264CRF = 52 }, false},
		{"relative transcode folder", func(s *ServerSettings) { s.Transcoding.TranscodeDir = "tmp" }, false},
		{"Windows transcode folder", func(s *ServerSettings) { s.Transcoding.TranscodeDir = `D:\transcodes` }, true},
		{"downmix boost", func(s *ServerSettings) { s.Transcoding.DownmixBoost = 4 }, false},
		{"catalog", func(s *ServerSettings) { s.PluginCatalogs = []string{"https://example.com/catalog.json"} }, true},
		{"metadata language and country", func(s *ServerSettings) { s.Metadata = MetadataSettings{PreferredLanguage: "de", Country: "DE"} }, true},
		{"no metadata language or country", func(s *ServerSettings) { s.Metadata = MetadataSettings{} }, true},
		{"metadata language of a region", func(s *ServerSettings) { s.Metadata.PreferredLanguage = "en-US" }, false},
		{"lower-case metadata country", func(s *ServerSettings) { s.Metadata.Country = "de" }, false},
		{"catalog without a scheme", func(s *ServerSettings) { s.PluginCatalogs = []string{"example.com/catalog.json"} }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := DefaultServerSettings()
			tt.change(&s)
			err := s.Validate()
			if (err == nil) != tt.ok || err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate() = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

func TestSeverityAtLeast(t *testing.T) {
	if !SeverityError.AtLeast(SeverityWarning) || SeverityInfo.AtLeast(SeverityWarning) || !SeverityInfo.AtLeast(SeverityInfo) {
		t.Error("AtLeast")
	}
}

func TestAccountSettings(t *testing.T) {
	tests := []struct {
		name    string
		a       AccountSettings
		ok      bool
		claimed bool
	}{
		{"unset", AccountSettings{}, true, false},
		{"standalone", AccountSettings{Mode: AccountModeStandalone}, true, false},
		{"unclaimed", AccountSettings{Mode: AccountModeMavioDev, Cursor: 3}, true, false},
		{"claimed", AccountSettings{Mode: AccountModeMavioDev, Administrator: "acct", Cursor: 3}, true, true},
		{"unknown mode", AccountSettings{Mode: "ldap"}, false, false},
		{"claimed standalone", AccountSettings{Mode: AccountModeStandalone, Administrator: "acct"}, false, false},
		{"standalone with a cursor", AccountSettings{Mode: AccountModeStandalone, Cursor: 1}, false, false},
		{"negative cursor", AccountSettings{Mode: AccountModeMavioDev, Cursor: -1}, false, false},
	}
	for _, tt := range tests {
		if err := tt.a.Validate(); (err == nil) != tt.ok || err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ok = %v", tt.name, err, tt.ok)
		}
		if got := tt.a.Claimed(); got != tt.claimed {
			t.Errorf("%s: Claimed = %v, want = %v", tt.name, got, tt.claimed)
		}
	}
}
