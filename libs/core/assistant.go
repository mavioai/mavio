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
	// Hints gives recognizers the user's hints.
	Hints bool
	// Speech is the recognizer and the voice of voice sessions by
	// language; a language listed nowhere, nor its base, gets the voice
	// session plugin's first ready ones.
	Speech []AssistantSpeech
	// LongestPause is the longest pause in a turn the turn model reads as
	// unfinished, half a second to ten seconds.
	LongestPause time.Duration
	// CommitDelay is how long a turn stands, with the user silent, before
	// what its answer changes is done, up to three seconds.
	CommitDelay time.Duration
	// SessionIdle is how long the user may stay silent after an answer
	// before the voice session ends, 3–120 seconds.
	SessionIdle time.Duration
}

// AssistantSpeech is the recognizer and the voice of a language.
type AssistantSpeech struct {
	// Language is BCP 47, such as "en" or "zh-CN".
	Language string
	// Recognizer is "<plugin>/<recognizer>", Voice "<plugin>/<voice>";
	// empty, the plugin's first ready one.
	Recognizer, Voice string
}

// SpeechOf returns the recognizer and the voice of a language, else of
// its base language.
func (s *AssistantSettings) SpeechOf(language string) AssistantSpeech {
	base, _, _ := strings.Cut(language, "-")
	var out AssistantSpeech
	for _, sp := range s.Speech {
		if strings.EqualFold(sp.Language, language) {
			return sp
		}
		if strings.EqualFold(sp.Language, base) {
			out = sp
		}
	}
	return out
}

// DefaultAssistantSettings are the settings of a new server: the
// assistant is on for everyone, with the AI path once a model is chosen.
func DefaultAssistantSettings() AssistantSettings {
	return AssistantSettings{
		Enabled: true, CodeMode: true, AIAccess: AssistantAccessAll, IdleTime: 30 * time.Minute,
		RetentionDays: 30, Memory: true, Hints: true,
		LongestPause: 3 * time.Second, CommitDelay: 700 * time.Millisecond, SessionIdle: 8 * time.Second,
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
	case s.LongestPause < 500*time.Millisecond || s.LongestPause > 10*time.Second:
		return fmt.Errorf("%w: longest pause %s outside half a second to ten seconds", ErrInvalid, s.LongestPause)
	case s.CommitDelay < 0 || s.CommitDelay > 3*time.Second:
		return fmt.Errorf("%w: commit delay %s outside zero to three seconds", ErrInvalid, s.CommitDelay)
	case s.SessionIdle < 3*time.Second || s.SessionIdle > 2*time.Minute:
		return fmt.Errorf("%w: session idle time %s outside 3–120 seconds", ErrInvalid, s.SessionIdle)
	}
	seen := map[string]bool{}
	for _, sp := range s.Speech {
		switch {
		case sp.Language == "" || seen[strings.ToLower(sp.Language)]:
			return fmt.Errorf("%w: speech of language %q given twice or without it", ErrInvalid, sp.Language)
		case sp.Recognizer != "" && !strings.Contains(sp.Recognizer, "/"):
			return fmt.Errorf("%w: recognizer %q is not <plugin>/<recognizer>", ErrInvalid, sp.Recognizer)
		case sp.Voice != "" && !strings.Contains(sp.Voice, "/"):
			return fmt.Errorf("%w: voice %q is not <plugin>/<voice>", ErrInvalid, sp.Voice)
		}
		seen[strings.ToLower(sp.Language)] = true
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
