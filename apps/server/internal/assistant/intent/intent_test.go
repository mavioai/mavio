package intent_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/mavioai/mavio/apps/server/internal/assistant/intent"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
)

// testCase is a case of libs/intent/testdata/cases, which the crate's own
// tests run too: the server's host must agree with them.
type testCase struct {
	Text         string         `yaml:"text"`
	Languages    []string       `yaml:"languages"`
	Confirmation bool           `yaml:"confirmation"`
	Results      int32          `yaml:"results"`
	Intent       *string        `yaml:"intent"`
	Slots        map[string]any `yaml:"slots"`
	Partial      bool           `yaml:"partial"`
}

func load(t *testing.T) *intent.Grammar {
	t.Helper()
	g, err := intent.Load(t.Context())
	if errors.Is(err, intent.ErrMissing) {
		t.Skip("the grammar is not built; run nx run intent:wasm")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Close(t.Context()) })
	return g
}

func TestCases(t *testing.T) {
	g := load(t)
	langs, err := g.Languages(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, lang := range langs.GetLanguages() {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "libs", "intent", "testdata", "cases", lang+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var cases []testCase
		if err := yaml.Unmarshal(b, &cases); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		for _, c := range cases {
			total++
			if err := check(t, g, lang, c); err != nil {
				t.Errorf("%s: %q: %v", lang, c.Text, err)
			}
		}
	}
	if total == 0 {
		t.Fatal("no cases")
	}
}

func check(t *testing.T, g *intent.Grammar, lang string, c testCase) error {
	languages := c.Languages
	if len(languages) == 0 {
		languages = []string{lang}
	}
	req := intentv1.ParseIntentRequest_builder{Text: &c.Text, Languages: languages}.Build()
	if c.Confirmation || c.Results > 0 {
		req.SetState(intentv1.DialogState_builder{Confirmation: &c.Confirmation, Results: &c.Results}.Build())
	}
	res, err := g.Parse(t.Context(), req)
	if err != nil {
		return err
	}
	if c.Intent == nil {
		if len(res.GetIntents()) > 0 {
			return fmt.Errorf("got = %s, want = none", res.GetIntents()[0].GetName())
		}
		return nil
	}
	if len(res.GetIntents()) == 0 {
		return fmt.Errorf("got = none, want = %s", *c.Intent)
	}
	got := res.GetIntents()[0]
	if got.GetName() != *c.Intent {
		return fmt.Errorf("got = %s, want = %s", got.GetName(), *c.Intent)
	}
	if partial := got.GetMatch() == intentv1.IntentMatch_INTENT_MATCH_PARTIAL; partial != c.Partial {
		return fmt.Errorf("partial: got = %v, want = %v", partial, c.Partial)
	}
	for name, want := range c.Slots {
		var s *intentv1.Slot
		for _, x := range got.GetSlots() {
			if x.GetName() == name {
				s = x
			}
		}
		if s == nil {
			return fmt.Errorf("slot %s: got = none, want = %v", name, want)
		}
		if !slotIs(s, want) {
			return fmt.Errorf("slot %s: got = %q %v, want = %v", name, s.GetText(), s, want)
		}
	}
	return nil
}

func slotIs(s *intentv1.Slot, want any) bool {
	switch w := want.(type) {
	case int:
		return slotIs(s, float64(w))
	case float64:
		switch {
		case s.HasNumber():
			return math.Abs(s.GetNumber()-w) < 1e-9
		case s.HasSeconds():
			return math.Abs(s.GetSeconds()-w) < 1e-9
		case s.HasOrdinal():
			return float64(s.GetOrdinal()) == w
		}
	case string:
		switch {
		case s.HasLanguage():
			return s.GetLanguage() == w
		case s.HasKeyword():
			return s.GetKeyword() == w
		case !s.HasNumber() && !s.HasSeconds() && !s.HasOrdinal():
			return s.GetText() == w
		}
	}
	return false
}

func TestReplyAndExamples(t *testing.T) {
	g := load(t)
	for _, c := range []struct{ language, key, title, want string }{
		{"en-US", "playing", "Dune", "Playing Dune."},
		{"zh-TW", "resumed", "", "繼續播放。"},
	} {
		r, err := g.Reply(t.Context(), intentv1.ReplyRequest_builder{
			Language: &c.language, Key: &c.key, Args: map[string]string{"title": c.title},
		}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if r.GetText() != c.want {
			t.Errorf("Reply(%s, %s) = %q, want = %q", c.language, c.key, r.GetText(), c.want)
		}
	}
	ex, err := g.Examples(t.Context(), intentv1.ExamplesRequest_builder{Language: new("de")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if ex.GetLanguage() != "de" || len(ex.GetExamples()) == 0 {
		t.Errorf("Examples(de) = %v, want = German examples", ex)
	}
}

func TestConcurrent(t *testing.T) {
	g := load(t)
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			var err error
			for range 20 {
				var r *intentv1.ParseIntentResponse
				if r, err = g.Parse(t.Context(), intentv1.ParseIntentRequest_builder{Text: new("pause"), Languages: []string{"en"}}.Build()); err == nil && r.GetIntents()[0].GetName() != "pause" {
					err = fmt.Errorf("got = %v, want = pause", r)
				}
				if err != nil {
					break
				}
			}
			errs <- err
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
