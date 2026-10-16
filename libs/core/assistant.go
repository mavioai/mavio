package core

import (
	"fmt"
	"strings"
	"time"
)

// AssistantAccess is who may use the assistant's AI path.
type AssistantAccess string

// Assistant accesses.
const (
	AssistantAccessAll            AssistantAccess = "all"
	AssistantAccessAdministrators AssistantAccess = "administrators"
	// AssistantAccessSelected lets the users of AssistantSettings.AIUsers.
	AssistantAccessSelected AssistantAccess = "selected"
)

// AssistantSettings are the server's assistant settings, part of the
// server settings.
type AssistantSettings struct {
	// Enabled lets users use the assistant.
	Enabled bool
	// Model is the AI path's model, "ai/<provider>/<model>"; empty, the
	// assistant has the fast path only.
	Model string
	// CodeMode lets the model write scripts calling the tools.
	CodeMode bool
	// AIAccess is who may use the AI path, AIUsers whom it selects.
	AIAccess AssistantAccess
	AIUsers  []ID
	// IdleTime is how long a conversation may be idle before a request
	// starts a new one, a minute to a day.
	IdleTime time.Duration
	// RetentionDays is how long conversations are kept, 1–3650 days.
	RetentionDays int
	// Memory lets the assistant remember what users tell about themselves.
	Memory bool
	// Recognizer is the server's recognizer, "<plugin>/<recognizer>";
	// empty, the first ready one.
	Recognizer string
	// Voices are the default voices, "<plugin>/<voice>", by BCP 47
	// language.
	Voices map[string]string
	// Hints gives recognizers the user's hints.
	Hints bool
}

// DefaultAssistantSettings are the settings of a new server: the
// assistant is on for everyone, with the AI path once a model is chosen.
func DefaultAssistantSettings() AssistantSettings {
	return AssistantSettings{
		Enabled: true, CodeMode: true, AIAccess: AssistantAccessAll, IdleTime: 30 * time.Minute,
		RetentionDays: 30, Memory: true, Hints: true,
	}
}

// Validate checks the settings' invariants.
func (s *AssistantSettings) Validate() error {
	switch {
	case s.Model != "" && !validModelRef(s.Model):
		return fmt.Errorf("%w: assistant model %q is not ai/<provider>/<model>", ErrInvalid, s.Model)
	case s.AIAccess != AssistantAccessAll && s.AIAccess != AssistantAccessAdministrators && s.AIAccess != AssistantAccessSelected:
		return fmt.Errorf("%w: unknown assistant access %q", ErrInvalid, s.AIAccess)
	case s.IdleTime < time.Minute || s.IdleTime > 24*time.Hour:
		return fmt.Errorf("%w: assistant idle time %s outside a minute to a day", ErrInvalid, s.IdleTime)
	case s.RetentionDays < 1 || s.RetentionDays > 3650:
		return fmt.Errorf("%w: assistant retention %d days outside 1–3650", ErrInvalid, s.RetentionDays)
	case s.Recognizer != "" && !strings.Contains(s.Recognizer, "/"):
		return fmt.Errorf("%w: recognizer %q is not <plugin>/<recognizer>", ErrInvalid, s.Recognizer)
	}
	for lang, v := range s.Voices {
		if lang == "" || !strings.Contains(v, "/") {
			return fmt.Errorf("%w: voice %q of %q is not <plugin>/<voice>", ErrInvalid, v, lang)
		}
	}
	return nil
}

// validModelRef reports whether s is "ai/<provider>/<model>".
func validModelRef(s string) bool {
	rest, ok := strings.CutPrefix(s, "ai/")
	if !ok {
		return false
	}
	provider, model, ok := strings.Cut(rest, "/")
	return ok && provider != "" && model != ""
}

// AllowsAI reports whether a user may use the AI path.
func (s *AssistantSettings) AllowsAI(u *User) bool {
	if !s.Enabled || s.Model == "" {
		return false
	}
	switch s.AIAccess {
	case AssistantAccessAll:
		return true
	case AssistantAccessAdministrators:
		return u.IsAdmin()
	default:
		for _, id := range s.AIUsers {
			if id == u.ID {
				return true
			}
		}
		return false
	}
}
