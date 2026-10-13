package core

import (
	"errors"
	"testing"
	"time"
)

func TestSubtitleLanguage(t *testing.T) {
	for _, tc := range []struct {
		lang  SubtitleLanguage
		str   string
		valid bool
	}{
		{SubtitleLanguage{Language: "en"}, "en", true},
		{SubtitleLanguage{Language: "pt-BR", HearingImpaired: true}, "pt-BR:hi", true},
		{SubtitleLanguage{Language: "zh-Hant", Forced: true}, "zh-Hant:forced", true},
		{SubtitleLanguage{Language: "es-419"}, "es-419", true},
		{SubtitleLanguage{Language: "en", Forced: true, HearingImpaired: true}, "en:forced", false},
		{SubtitleLanguage{}, "", false},
		{SubtitleLanguage{Language: "419"}, "419", false},
		{SubtitleLanguage{Language: "en-"}, "en-", false},
		{SubtitleLanguage{Language: "en us"}, "en us", false},
	} {
		if got := tc.lang.String(); got != tc.str {
			t.Errorf("%+v.String() got = %q, want = %q", tc.lang, got, tc.str)
		}
		if err := tc.lang.Validate(); (err == nil) != tc.valid {
			t.Errorf("%+v.Validate() got = %v, want valid = %v", tc.lang, err, tc.valid)
		}
	}
}

func TestSubtitleProfileValidate(t *testing.T) {
	valid := func() SubtitleProfile {
		return SubtitleProfile{
			ID: NewID(), Name: "Default", Cutoff: 1,
			Items: []SubtitleProfileItem{
				{ID: 1, Language: "en"},
				{ID: 2, Language: "en", HI: SubtitleHIRequired},
				{ID: 3, Language: "fr", Forced: true, AudioOnlyInclude: true},
			},
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*SubtitleProfile)
		valid  bool
	}{
		{"valid", func(*SubtitleProfile) {}, true},
		{"cutoff any", func(p *SubtitleProfile) { p.Cutoff = SubtitleCutoffAny }, true},
		{"no cutoff", func(p *SubtitleProfile) { p.Cutoff = 0 }, true},
		{"no ID", func(p *SubtitleProfile) { p.ID = NilID }, false},
		{"no name", func(p *SubtitleProfile) { p.Name = " " }, false},
		{"no items", func(p *SubtitleProfile) { p.Items = nil }, false},
		{"tag with a space", func(p *SubtitleProfile) { p.Tag = "a b" }, false},
		{"cutoff of no item", func(p *SubtitleProfile) { p.Cutoff = 4 }, false},
		{"item ID twice", func(p *SubtitleProfile) { p.Items[1].ID = 1 }, false},
		{"item ID 0", func(p *SubtitleProfile) { p.Items[0].ID = 0 }, false},
		{"language twice", func(p *SubtitleProfile) { p.Items[1].HI = SubtitleHIAllowed }, false},
		{"bad language", func(p *SubtitleProfile) { p.Items[0].Language = "" }, false},
		{"bad HI", func(p *SubtitleProfile) { p.Items[0].HI = "maybe" }, false},
		{"forced HI", func(p *SubtitleProfile) { p.Items[2].HI = SubtitleHIRequired }, false},
		{"audio both", func(p *SubtitleProfile) { p.Items[2].AudioExclude = true }, false},
	} {
		p := valid()
		tc.change(&p)
		err := p.Validate()
		if (err == nil) != tc.valid || err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got = %v, want valid = %v", tc.name, err, tc.valid)
		}
	}
}

func TestSubtitleHistoryEventValidate(t *testing.T) {
	id := NewID()
	ok := SubtitleHistoryEvent{ID: id, Action: SubtitleSynced, ItemID: NewID(), Date: time.Now(), Language: SubtitleLanguage{Language: "en"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid event: got = %v, want = nil", err)
	}
	self := ok
	self.UpgradedFrom = id
	if err := self.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("event upgrading itself: got = %v, want = ErrInvalid", err)
	}
}

func TestSubtitleSettingsValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*SubtitleSettings)
		valid  bool
	}{
		{"defaults", func(*SubtitleSettings) {}, true},
		{"relative folder", func(s *SubtitleSettings) { s.Folder, s.CustomFolder = SubtitleFolderRelative, "Subs" }, true},
		{"metadata folder", func(s *SubtitleSettings) { s.Folder = SubtitleFolderMetadata }, true},
		{"searches off", func(s *SubtitleSettings) { s.WantedSearchInterval = 0 }, true},
		{"equivalence", func(s *SubtitleSettings) {
			s.LanguageEquals = []SubtitleLanguageEquivalence{{From: SubtitleLanguage{Language: "en", HearingImpaired: true}, To: SubtitleLanguage{Language: "en"}}}
		}, true},
		{"score over 100", func(s *SubtitleSettings) { s.MinimumScore = 101 }, false},
		{"HI extension", func(s *SubtitleSettings) { s.HIExtension = "deaf" }, false},
		{"unknown folder", func(s *SubtitleSettings) { s.Folder = "elsewhere" }, false},
		{"relative folder escaping", func(s *SubtitleSettings) { s.Folder, s.CustomFolder = SubtitleFolderRelative, "../subs" }, false},
		{"relative folder absolute", func(s *SubtitleSettings) { s.Folder, s.CustomFolder = SubtitleFolderRelative, "/subs" }, false},
		{"metadata folder named", func(s *SubtitleSettings) { s.Folder, s.CustomFolder = SubtitleFolderMetadata, "/subs" }, false},
		{"relative folder nested", func(s *SubtitleSettings) { s.Folder, s.CustomFolder = SubtitleFolderRelative, "Subs/en" }, false},
		{"upgrade days", func(s *SubtitleSettings) { s.UpgradeDays = 31 }, false},
		{"interval", func(s *SubtitleSettings) { s.UpgradeInterval = time.Minute }, false},
		{"default language", func(s *SubtitleSettings) { s.DefaultAudioLanguage = "e n" }, false},
		{"empty mod", func(s *SubtitleSettings) { s.Mods = []string{""} }, false},
		{"tag", func(s *SubtitleSettings) { s.NoProfileTags = []string{""} }, false},
		{"bad equivalence", func(s *SubtitleSettings) {
			s.LanguageEquals = []SubtitleLanguageEquivalence{{From: SubtitleLanguage{Language: "en"}}}
		}, false},
		{"sync threshold", func(s *SubtitleSettings) { s.Sync.Threshold = -1 }, false},
		{"sync offset", func(s *SubtitleSettings) { s.Sync.MaxOffset = 0 }, false},
		{"sync deviation", func(s *SubtitleSettings) { s.Sync.MaxScaleDeviation = -0.1 }, false},
	} {
		s := DefaultSubtitleSettings()
		tc.change(&s)
		err := s.Validate()
		if (err == nil) != tc.valid || err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got = %v, want valid = %v", tc.name, err, tc.valid)
		}
	}
}
