package subtitles

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/subtitle"
)

// fakeAI is an AI gateway with the provider "p" and its model "m", whose
// completions answer gives.
type fakeAI struct {
	pluginv1connect.AiGatewayServiceClient

	mu       sync.Mutex
	noKey    bool
	answer   func(req *pluginv1.CompleteAiRequest) (string, error)
	requests []*pluginv1.CompleteAiRequest
}

func (f *fakeAI) ListAiModels(_ context.Context, req *pluginv1.ListAiModelsRequest) (*pluginv1.ListAiModelsResponse, error) {
	if req.GetProvider() != "p" {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no provider"))
	}
	if req.GetAvailable() && f.noKey {
		return &pluginv1.ListAiModelsResponse{}, nil
	}
	return pluginv1.ListAiModelsResponse_builder{Models: []*pluginv1.AiModel{
		pluginv1.AiModel_builder{Provider: proto.String("p"), Id: proto.String("m"), Name: proto.String("Model M")}.Build(),
	}}.Build(), nil
}

func (f *fakeAI) CompleteAi(_ context.Context, req *pluginv1.CompleteAiRequest) (*pluginv1.CompleteAiResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	text, err := f.answer(req)
	if err != nil {
		return nil, err
	}
	resp := pluginv1.CompleteAiResponse_builder{
		Usage: pluginv1.AiUsage_builder{InputTokens: proto.Int64(10), OutputTokens: proto.Int64(5), Cost: proto.Float64(0.5)}.Build(),
	}.Build()
	if req.GetOutputSchema() != "" {
		resp.SetOutputJson(text)
	} else {
		resp.SetText(text)
	}
	return resp, nil
}

func (f *fakeAI) take() []*pluginv1.CompleteAiRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func TestTranslateAI(t *testing.T) {
	e := newEnv(t)
	ai := &fakeAI{}
	e.rt.AI = func() (pluginv1connect.AiGatewayServiceClient, bool) { return ai, true }
	ts := &e.s.Translation
	ts.Translator, ts.RetryDelay = "ai/p/m", 0
	ts.ContextBefore, ts.ContextAfter, ts.TranslatedContext = 1, 1, true
	ts.UserPrompt = "{contextBefore}|{lineToTranslate}|{contextAfter}"
	en, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\nthere\n\n"+
		"2\n00:00:03,000 --> 00:00:04,000\n<i>Bye</i>\n\n3\n00:00:03,000 --> 00:00:04,000\n<i>Bye</i>\n"), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	fr := core.SubtitleLanguage{Language: "fr"}

	for _, c := range []struct {
		name       string
		translator string
		noKey      bool
		want       error
	}{
		{"unknown provider", "ai/q/m", false, core.ErrNotFound},
		{"unknown model", "ai/p/x", false, core.ErrNotFound},
		{"no credential", "ai/p/m", true, ErrUnavailable},
	} {
		ai.noKey = c.noKey
		if _, err := e.rt.QueueTranslate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr, Translator: c.translator}); !errors.Is(err, c.want) {
			t.Errorf("QueueTranslate(%s): err = %v, want %v", c.name, err, c.want)
		}
	}
	ai.noKey = false
	e.rt.AI = nil
	if _, err := e.rt.QueueTranslate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("QueueTranslate without an AI gateway: err = %v, want ErrUnavailable", err)
	}
	e.rt.AI = func() (pluginv1connect.AiGatewayServiceClient, bool) { return ai, true }

	// Cue by cue, with the cues around as context and the translation of
	// the one before; the stacked cue is translated once, and a rate
	// limit is retried.
	limited := false
	ai.answer = func(req *pluginv1.CompleteAiRequest) (string, error) {
		if !limited {
			limited = true
			return "", connect.NewError(connect.CodeUnavailable, errors.New("rate limited"))
		}
		parts := strings.Split(req.GetMessages()[0].GetText(), "|")
		return strings.ToUpper(parts[1]), nil
	}
	if _, err := e.rt.Translate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr}); err != nil {
		t.Fatal(err)
	}
	reqs := ai.take()
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3: a rate limited one and two cues", len(reqs))
	}
	if r := reqs[1]; r.GetProvider() != "p" || r.GetModel() != "m" || !strings.HasPrefix(r.GetSystem(), "Translate from English to French,") ||
		r.GetMessages()[0].GetText() != `|Hello there|{"position":2,"line":"<i>Bye</i>"}` {
		t.Errorf("first cue's request = %v", r)
	}
	if got, want := reqs[2].GetMessages()[0].GetText(), `{"position":1,"line":"Hello there","translation":"HELLO THERE"}|<i>Bye</i>|{"position":3,"line":"<i>Bye</i>"}`; got != want {
		t.Errorf("second cue's message = %q, want = %q", got, want)
	}
	want := "1\n00:00:00,000 --> 00:00:01,000\n# Subtitles translated with Model M # \n\n2\n00:00:01,000 --> 00:00:02,000\nHELLO THERE\n\n" +
		"3\n00:00:03,000 --> 00:00:04,000\n<I>BYE</I>\n\n4\n00:00:03,000 --> 00:00:04,000\n<I>BYE</I>\n"
	if got := e.file("The Matrix (1999).fr.srt"); strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Errorf("translation = %q, want = %q", got, want)
	}
	h := e.history(core.SubtitleTranslated)
	wantRecord := &core.SubtitleTranslationRecord{
		SourcePath: en, SourceLanguage: "en", Translators: []string{"ai/p/m"}, Cues: 3, Characters: 21,
		InputTokens: 20, OutputTokens: 10, Cost: 1,
	}
	if len(h) != 1 || !reflect.DeepEqual(h[0].Translation, wantRecord) || h[0].Description != "en subtitles translated to fr with Model M." {
		t.Errorf("history = %+v, want the translation recorded with %+v", h, wantRecord)
	}

	// Proofreading corrects the cues whose translation the model changes.
	frPath := h[0].Path
	ai.answer = func(req *pluginv1.CompleteAiRequest) (string, error) {
		if !strings.Contains(req.GetSystem(), "proofreading a subtitle translated from English to French") {
			return "", errors.New("not proofreading")
		}
		if req.GetMessages()[0].GetText() == "[SOURCE] Hello there\n[TRANSLATION] HELLO THERE" {
			return "Bonjour", nil
		}
		return strings.TrimPrefix(req.GetMessages()[0].GetText()[strings.Index(req.GetMessages()[0].GetText(), "[TRANSLATION] "):], "[TRANSLATION] "), nil
	}
	if _, err := e.rt.QueueProofread(e.ctx, ProofreadPayload{ItemID: e.movie.ID, Path: en}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("QueueProofread of a subtitle not translated: err = %v, want ErrInvalid", err)
	}
	if _, err := e.rt.QueueProofread(e.ctx, ProofreadPayload{ItemID: e.movie.ID, Path: frPath, Translator: "plug/upper"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("QueueProofread with a plugin translator: err = %v, want ErrInvalid", err)
	}
	if _, err := e.rt.QueueProofread(e.ctx, ProofreadPayload{ItemID: e.movie.ID, Path: frPath}); err != nil {
		t.Fatal(err)
	}
	revised, err := e.rt.Proofread(e.ctx, ProofreadPayload{ItemID: e.movie.ID, Path: frPath})
	if err != nil || revised != 1 {
		t.Fatalf("Proofread = %d, %v; want 1 cue revised", revised, err)
	}
	if got := e.file("The Matrix (1999).fr.srt"); !strings.Contains(got, "\nBonjour\n") || !strings.Contains(got, "<I>BYE</I>") {
		t.Errorf("proofread translation = %q", got)
	}
	if h := e.history(core.SubtitleTranslated); !h[0].Translation.Proofread || h[0].Translation.Revised != 1 || h[0].Translation.InputTokens != 50 {
		t.Errorf("record after proofreading = %+v, want proofread with 1 cue revised and its tokens", h[0].Translation)
	}

	// In batches answered as JSON, a cue missing keeps its text; a
	// translator failing falls back to the next.
	ts.Batch, ts.StripFormatting, ts.Credit = true, true, false
	ts.Translator, ts.FallbackTranslators = "ai/p/x", []string{"ai/p/m"}
	if _, err := e.rt.Translate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: fr}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Translate with an unknown first translator: err = %v, want not found", err)
	}
	tr := &fakeTranslator{enabled: true}
	e.rt.Translators = func() []Translator { return []Translator{{ID: "plug", Client: tr}} }
	ts.Translator = "plug/upper"
	failing := true
	ai.answer = func(req *pluginv1.CompleteAiRequest) (string, error) {
		if req.GetOutputSchema() == "" || req.GetMessages()[0].GetText() != `[{"position":1,"line":"Hello there"},{"position":2,"line":"Bye"}]` {
			return "", errors.New("unexpected request")
		}
		return `{"translations":[{"position":1,"line":"Salut"},{"position":1,"line":"toi"}]}`, nil
	}
	tr.fail = &failing
	de := core.SubtitleLanguage{Language: "de"}
	e.s.Translation.FallbackTranslators = []string{"ai/p/m"}
	if _, err := e.rt.Translate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: de}); err != nil {
		t.Fatal(err)
	}
	if got := e.file("The Matrix (1999).de.srt"); !strings.Contains(got, "Salut\ntoi") || !strings.Contains(got, "\nBye\n") || strings.Contains(got, "<i>") {
		t.Errorf("batch translation = %q, want the cue missing kept and no markup", got)
	}
	if h := e.history(core.SubtitleTranslated); !reflect.DeepEqual(h[0].Translation.Translators, []string{"ai/p/m"}) {
		t.Errorf("translators = %v, want the fallback", h[0].Translation.Translators)
	}

	st, err := e.rt.TranslationStatistics(e.ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st.Files != 2 || st.Cues != 6 || st.ByTranslator["ai/p/m"] != 2 || st.ByLanguagePair["en:fr"] != 1 || st.ByLanguagePair["en:de"] != 1 ||
		st.ByMedia["movie"] != 2 || len(st.Daily) != 3 || st.Daily[2].Files != 2 || st.Daily[2].Date != "2026-10-01" {
		t.Errorf("statistics = %+v", st)
	}
}

func TestTranslateWanted(t *testing.T) {
	e := newEnv(t)
	tr := &fakeTranslator{enabled: true}
	e.rt.Translators = func() []Translator { return []Translator{{ID: "plug", Client: tr}} }
	a := &e.s.Translation.Automation
	a.Enabled, a.SourceLanguages, a.MovieAge = true, []string{"de", "en"}, time.Hour
	en, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	pending := func() []core.Job {
		t.Helper()
		var jobs []core.Job
		for {
			job, err := e.db.Jobs().Lease(e.ctx, "test", []string{JobTranslate}, time.Minute)
			if errors.Is(err, core.ErrNotFound) {
				return jobs
			}
			if err != nil {
				t.Fatal(err)
			}
			jobs = append(jobs, job)
		}
	}
	// The movie was added too lately.
	e.movie.DateAdded = e.rt.now().Add(-time.Minute)
	if err := e.db.Items().Upsert(e.ctx, e.movie); err != nil {
		t.Fatal(err)
	}
	if err := e.rt.TranslateWanted(e.ctx); err != nil {
		t.Fatal(err)
	}
	if jobs := pending(); len(jobs) != 0 {
		t.Errorf("jobs for a movie added a minute ago = %d, want none", len(jobs))
	}
	a.MovieAge = 0
	for range 2 {
		if err := e.rt.TranslateWanted(e.ctx); err != nil {
			t.Fatal(err)
		}
	}
	jobs := pending()
	if len(jobs) != 1 || !strings.Contains(string(jobs[0].Payload), `"language":{"Language":"fr"`) || !strings.Contains(string(jobs[0].Payload), en) {
		t.Errorf("jobs = %+v, want one translating the English file to French", jobs)
	}
	if got := e.rt.Tasks(); got[len(got)-1].Kind != JobTranslateWanted || got[len(got)-1].Interval != 24*time.Hour {
		t.Errorf("tasks = %+v, want the automation daily", got)
	}
}

func TestPrompt(t *testing.T) {
	got := fillPrompt("{sourceLanguage} to {targetLanguage}: {lineToTranslate} {unknown}", map[string]string{
		"sourceLanguage": languageName("en", false), "targetLanguage": languageName("pt-BR", false), "lineToTranslate": "Hi",
	})
	if want := "English to Brazilian Portuguese: Hi {unknown}"; got != want {
		t.Errorf("fillPrompt = %q, want = %q", got, want)
	}
	if got := languageName("zh-Hant", true); got != "zh-Hant" {
		t.Errorf("languageName(codes) = %q, want zh-Hant", got)
	}
}

func TestMatchLanguage(t *testing.T) {
	for _, c := range []struct {
		want      string
		languages []string
		match     string
		tier      matchTier
	}{
		{"en", nil, "en", matchExact},
		{"pt-BR", []string{"pt", "pt-br"}, "pt-br", matchExact},
		{"he", []string{"iw"}, "iw", matchAlias},
		{"zh-TW", []string{"zh-Hans", "zh-Hant"}, "zh-Hant", matchScript},
		{"pt-BR", []string{"pt-PT", "pt"}, "pt", matchNeutral},
		{"pt-BR", []string{"pt-PT"}, "", matchNone},
		{"en", []string{"en-GB", "en-US"}, "en-GB", matchVariant},
		{"fr", []string{"de"}, "", matchNone},
	} {
		if m, tier := matchLanguage(c.want, c.languages); m != c.match || tier != c.tier {
			t.Errorf("matchLanguage(%s, %v) = %q, %d; want = %q, %d", c.want, c.languages, m, tier, c.match, c.tier)
		}
	}
}

func TestSubtitleLines(t *testing.T) {
	for _, c := range []struct {
		translated      string
		lines           int
		preserve, strip bool
		want            []string
	}{
		{"one\ntwo", 2, true, false, []string{"one", "two"}},
		{"one\ntwo\nthree", 2, true, false, []string{"one two three"}},
		{
			"a long translation that goes well beyond forty two characters", 2, false, true,
			[]string{"a long translation that goes well beyond", "forty two characters"},
		},
		{
			"a long translation that goes well beyond forty two characters", 2, false, false,
			[]string{"a long translation that goes well beyond forty two characters"},
		},
		{"short", 1, false, true, []string{"short"}},
	} {
		if got := subtitleLines(c.translated, c.lines, c.preserve, c.strip); !reflect.DeepEqual(got, c.want) {
			t.Errorf("subtitleLines(%q, %d, %v, %v) = %q, want = %q", c.translated, c.lines, c.preserve, c.strip, got, c.want)
		}
	}
}

func TestFixOverlaps(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	cues := []subtitle.Cue{
		{Start: ms(0), End: ms(2000)}, {Start: ms(1500), End: ms(3000)}, // trimmed to 1480
		{Start: ms(1700), End: ms(4000)}, // the one before would last under 500 ms
		{Start: ms(5000), End: ms(6000)},
	}
	fixOverlaps(cues)
	if got := []time.Duration{cues[0].End, cues[1].End, cues[2].End}; !reflect.DeepEqual(got, []time.Duration{ms(1480), ms(3000), ms(4000)}) {
		t.Errorf("ends = %v, want = [1.48s 3s 4s]", got)
	}
}

func TestValidateSubtitle(t *testing.T) {
	v := core.DefaultSubtitleSettings().Translation.Validation
	s := time.Second
	good := []subtitle.Cue{{Start: 0, End: s, Text: "Hello"}, {Start: 2 * s, End: 3 * s, Text: "Bye"}}
	for _, c := range []struct {
		name  string
		size  int
		cues  []subtitle.Cue
		valid bool
	}{
		{"good", 100, good, true},
		{"too big", 2 << 20, good, false},
		{"one cue", 100, good[:1], false},
		{"short cue", 100, []subtitle.Cue{good[0], {Start: 2 * s, End: 3 * s, Text: "a"}}, false},
		{"long duration", 100, []subtitle.Cue{good[0], {Start: 2 * s, End: 20 * s, Text: "Bye"}}, false},
		{"overlap", 100, []subtitle.Cue{good[0], {Start: s / 2, End: 3 * s, Text: "Bye"}}, false},
		{"control", 100, []subtitle.Cue{good[0], {Start: 2 * s, End: 3 * s, Text: "By\x07e"}}, false},
	} {
		if err := validateSubtitle(c.size, c.cues, &v); (err == nil) != c.valid {
			t.Errorf("validateSubtitle(%s) = %v, want valid = %v", c.name, err, c.valid)
		}
	}
}

func TestSourceCues(t *testing.T) {
	cue := func(start, end int, text string) subtitle.Cue {
		return subtitle.Cue{Start: time.Duration(start) * time.Second, End: time.Duration(end) * time.Second, Text: text}
	}
	source := []subtitle.Cue{cue(1, 3, "<i>One</i>"), cue(4, 6, "Two\nlines"), cue(7, 9, "Three")}
	for _, c := range []struct {
		name       string
		translated []subtitle.Cue
		want       []string
	}{
		{"synchronized since", []subtitle.Cue{cue(11, 13, "Un"), cue(14, 16, "Deux"), cue(17, 19, "Trois")}, []string{"One", "Two lines", "Three"}},
		{"credited", []subtitle.Cue{cue(0, 1, creditPrefix+"M # "), cue(1, 3, "Un"), cue(4, 6, "Deux"), cue(7, 9, "Trois")}, []string{"", "One", "Two lines", "Three"}},
		{"a cue removed, one trimmed", []subtitle.Cue{cue(1, 3, "Un"), cue(7, 8, "Trois")}, []string{"One", "Three"}},
		{"cues merged", []subtitle.Cue{cue(1, 6, "Un deux")}, []string{"One Two lines"}},
		{"nothing overlapping", []subtitle.Cue{cue(20, 21, "Vingt")}, []string{""}},
	} {
		if got := sourceCues(source, c.translated); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: sourceCues = %q, want = %q", c.name, got, c.want)
		}
	}
}
