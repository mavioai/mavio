package core

import (
	"errors"
	"testing"
	"time"
)

func TestAssistantSettingsValidate(t *testing.T) {
	for _, c := range []struct {
		name  string
		edit  func(*AssistantSettings)
		valid bool
	}{
		{"defaults", func(*AssistantSettings) {}, true},
		{"model", func(s *AssistantSettings) { s.Model = "ai/openai/gpt-5" }, true},
		{"model without provider", func(s *AssistantSettings) { s.Model = "ai/gpt-5" }, false},
		{"model of a plugin", func(s *AssistantSettings) { s.Model = "deepl/x" }, false},
		{"unknown access", func(s *AssistantSettings) { s.AIAccess = "some" }, false},
		{"idle too short", func(s *AssistantSettings) { s.IdleTime = time.Second }, false},
		{"idle too long", func(s *AssistantSettings) { s.IdleTime = 25 * time.Hour }, false},
		{"retention", func(s *AssistantSettings) { s.RetentionDays = 0 }, false},
		{"recognizer", func(s *AssistantSettings) { s.Recognizer = "whisper" }, false},
		{"voice", func(s *AssistantSettings) { s.Voices = map[string]string{"en": "voice/openai/alloy"} }, true},
		{"voice without plugin", func(s *AssistantSettings) { s.Voices = map[string]string{"en": "alloy"} }, false},
	} {
		s := DefaultAssistantSettings()
		c.edit(&s)
		err := s.Validate()
		if (err == nil) != c.valid || (err != nil && !errors.Is(err, ErrInvalid)) {
			t.Errorf("%s: Validate() = %v, want valid = %v", c.name, err, c.valid)
		}
	}
}

func TestAssistantSettingsAllowsAI(t *testing.T) {
	admin := User{ID: NewID(), Kind: UserAdmin}
	user := User{ID: NewID()}
	other := User{ID: NewID()}
	for _, c := range []struct {
		name string
		edit func(*AssistantSettings)
		user *User
		want bool
	}{
		{"no model", func(*AssistantSettings) {}, &admin, false},
		{"all", func(s *AssistantSettings) { s.Model = "ai/p/m" }, &user, true},
		{"off", func(s *AssistantSettings) { s.Model, s.Enabled = "ai/p/m", false }, &admin, false},
		{"administrators", func(s *AssistantSettings) { s.Model, s.AIAccess = "ai/p/m", AssistantAccessAdministrators }, &user, false},
		{"administrator", func(s *AssistantSettings) { s.Model, s.AIAccess = "ai/p/m", AssistantAccessAdministrators }, &admin, true},
		{"selected", func(s *AssistantSettings) {
			s.Model, s.AIAccess, s.AIUsers = "ai/p/m", AssistantAccessSelected, []ID{user.ID}
		}, &user, true},
		{"not selected", func(s *AssistantSettings) {
			s.Model, s.AIAccess, s.AIUsers = "ai/p/m", AssistantAccessSelected, []ID{user.ID}
		}, &other, false},
	} {
		s := DefaultAssistantSettings()
		c.edit(&s)
		if got := s.AllowsAI(c.user); got != c.want {
			t.Errorf("%s: AllowsAI() = %v, want = %v", c.name, got, c.want)
		}
	}
}
