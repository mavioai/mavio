package subtitles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"golang.org/x/text/language"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Test cases ported from Lingarr by tools/testport live in testdata/cases
// with Bazarr's; see docs/testing.md §2.

// scriptedAI is an AI gateway answering completions with answer.
type scriptedAI struct {
	pluginv1connect.AiGatewayServiceClient

	mu       sync.Mutex
	answer   func(ctx context.Context, req *pluginv1.CompleteAiRequest) (string, error)
	requests []*pluginv1.CompleteAiRequest
}

func (f *scriptedAI) CompleteAi(ctx context.Context, req *pluginv1.CompleteAiRequest) (*pluginv1.CompleteAiResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	text, err := f.answer(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &pluginv1.CompleteAiResponse{}
	if req.GetOutputSchema() != "" {
		resp.SetOutputJson(text)
	} else {
		resp.SetText(text)
	}
	return resp, nil
}

// call is a cue translation asked of a model, from the user prompt
// "{contextBefore}|{lineToTranslate}|{contextAfter}".
type call struct {
	text          string
	before, after []string
}

func (f *scriptedAI) calls() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, r := range f.requests {
		parts := strings.Split(r.GetMessages()[0].GetText(), "|")
		c := call{text: parts[1]}
		if parts[0] != "" {
			c.before = strings.Split(parts[0], "\n")
		}
		if parts[2] != "" {
			c.after = strings.Split(parts[2], "\n")
		}
		out = append(out, c)
	}
	return out
}

// lingarrSettings are the translation settings of Lingarr's tests: no
// context, line breaks not kept, formatting kept, retries at once.
func lingarrSettings() core.SubtitleTranslationSettings {
	s := core.DefaultSubtitleSettings().Translation
	s.ContextBefore, s.ContextAfter, s.RetryDelay = 0, 0, 0
	s.UserPrompt = "{contextBefore}|{lineToTranslate}|{contextAfter}"
	return s
}

// cue returns an item at a position, untimed as Lingarr's content lines.
func cue(position int, lines ...string) *cueItem {
	return &cueItem{position: position, lines: lines}
}

// timed returns an item at a position with a timing in milliseconds.
func timed(position, start, end int, lines ...string) *cueItem {
	return &cueItem{position: position, start: time.Duration(start) * time.Millisecond, end: time.Duration(end) * time.Millisecond, lines: lines}
}

// translateItems translates items from en to target with a model
// answering cues with translate, or batches with batch.
func translateItems(t *testing.T, s core.SubtitleTranslationSettings, target string, answer func(ctx context.Context, req *pluginv1.CompleteAiRequest) (string, error), items ...*cueItem) (*scriptedAI, error) {
	t.Helper()
	ai := &scriptedAI{answer: answer}
	c := chained{translator: &translator{name: "ai/p/m", credit: "M", ai: ai, provider: "p", model: "m"}, source: "en", target: target}
	tr := &translation{
		r: &Runtime{}, s: &s, source: "en", target: target, candidates: []chained{c},
		logger: slog.New(slog.DiscardHandler), items: items, record: &core.SubtitleTranslationRecord{},
	}
	return ai, tr.run(t.Context())
}

// perLine answers each cue's line with translate.
func perLine(translate func(text string) string) func(context.Context, *pluginv1.CompleteAiRequest) (string, error) {
	return func(_ context.Context, req *pluginv1.CompleteAiRequest) (string, error) {
		return translate(strings.Split(req.GetMessages()[0].GetText(), "|")[1]), nil
	}
}

// batchAnswer answers batches with translate's translations by position.
func batchAnswer(translate func(items []batchItem) []batchItem) func(context.Context, *pluginv1.CompleteAiRequest) (string, error) {
	return func(_ context.Context, req *pluginv1.CompleteAiRequest) (string, error) {
		var items []batchItem
		if err := json.Unmarshal([]byte(req.GetMessages()[0].GetText()), &items); err != nil {
			return "", err
		}
		return marshalJSON(map[string][]batchItem{"translations": translate(items)}), nil
	}
}

func contextJSON(position int, line, translation string) string {
	return marshalJSON(contextLine{Position: position, Line: line, Translation: translation})
}

func check[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want = %#v", what, got, want)
	}
}

const (
	noContentAPI = "Lingarr's /api/translate/content and /api/translate/line serve Bazarr; Mavio has Bazarr's subtitle acquisition " +
		"built in and translates the subtitle files of its library, with no API translating lines sent by clients. " +
		"The translation engine they share is ported from SubtitleTranslationServiceTests"
	piAI = "Mavio reaches AI providers through the agent plugin's pi-ai, whose tests cover the providers' requests; " +
		"the retries and answers around them are ported from the cases of the same behavior"
)

func TestPortedSubtitleTranslationService(t *testing.T) {
	portedCases(t, "subtitle_translation_service.json", ported{
		skip: map[string]string{
			"TranslateSubtitles_MergeStackedLinesOff_RepeatedTextIsTranslatedIndependentlyWithContext": "Mavio translates only subtitle " +
				"files, whose cues are timed, and always translates stacked cues of the same timing and text once; " +
				"Lingarr's option serves its content API",
		},
		facts: map[string]string{
			"TranslateSubtitles_SingleLine_TranslatesAsOneCallAndReturnsOneLine":                                 "TestLingarrTranslateLines",
			"TranslateSubtitles_MultiLinePreserveOff_MergesIntoOneCallAndReturnsOneLine":                         "TestLingarrTranslateLines",
			"TranslateSubtitles_MultiLinePreserveOffStripOn_RewrapsLongTranslation":                              "TestLingarrTranslateLines",
			"TranslateSubtitles_MultiLinePreserveOn_TranslatesEachLineSeparately":                                "TestLingarrTranslateLines",
			"TranslateSubtitles_PreserveOnPerLineTranslationWithEmbeddedNewline_KeepsModelOutputVerbatim":        "TestLingarrTranslateLines",
			"TranslateSubtitles_WhitespaceLine_SkipsTranslationAndPassesThrough":                                 "TestLingarrTranslateLines",
			"TranslateSubtitles_StackedDuplicates_TranslatesOnceAndReusesResult":                                 "TestLingarrStackedCues",
			"TranslateSubtitles_SameTextDifferentTimes_TranslatesEachIndependently":                              "TestLingarrStackedCues",
			"TranslateSubtitles_NoDuplicates_TranslatesEvery":                                                    "TestLingarrStackedCues",
			"TranslateSubtitles_SameTimestampDifferentText_TranslatesEachIndependently":                          "TestLingarrStackedCues",
			"TranslateSubtitles_MergeStackedLinesOn_UntimedRepeatedTextIsStillMerged":                            "TestLingarrStackedCues",
			"TranslateSubtitles_TranslatedContextOff_ContextIsSourceTextOnBothSides":                             "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOn_PreviouslyTranslatedLinesArePairedSentenceBySentence":        "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOn_ContextAfterIsStructuredWithoutTranslation":                  "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOn_ResumedLinesArePairedFromPersistedTranslation":               "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOnPreserveLineBreaks_MultiLinePairIsCollapsedToOneLineEachSide": "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOn_EmptyStoredTranslationIsPassedWithoutTranslationField":       "TestLingarrContext",
			"TranslateSubtitles_TranslatedContextOn_EmbeddedNewlineAndNonAsciiStayOnOneReadableLine":             "TestLingarrContext",
			"ProcessSubtitleBatch_PreserveOff_JoinsWithSpaceAndReturnsSingleLine":                                "TestLingarrBatch",
			"ProcessSubtitleBatch_PreserveOnMultiLine_JoinsWithNewlineAndSplitsBack":                             "TestLingarrBatch",
			"ProcessSubtitleBatch_PreserveOnButLineCountMismatch_CollapsesToSingleLine":                          "TestLingarrBatch",
			"ProcessSubtitleBatch_PreserveOffStripOnMultiLine_RewrapsLongTranslation":                            "TestLingarrBatch",
			"ProcessSubtitleBatch_MissingTranslation_FallsBackToOriginalLines":                                   "TestLingarrBatch",
			"TranslateSubtitleLine_PrefersExactMatchOverNeutralFallback":                                         "TestLingarrChain",
			"TranslateSubtitleLine_SameTierBreaksTiesByChainOrder":                                               "TestLingarrChain",
			"TranslateSubtitleLine_FallbackMatchSurfacesMatchedCodesOnEmittedLine":                               "TestLingarrChain",
			"TranslateSubtitleLine_NoServiceSupportsPair_Throws":                                                 "TestLingarrChain",
		},
	})
}

const longTranslation = "Esta es una traducción bastante larga que sin duda excederá el límite de cuarenta y dos caracteres"

func TestLingarrTranslateLines(t *testing.T) {
	s := lingarrSettings()
	it := cue(1, "hello")
	ai, err := translateItems(t, s, "es", perLine(func(string) string { return "hola" }), it)
	if err != nil {
		t.Fatal(err)
	}
	check(t, "single line", it.translated, []string{"hola"})
	check(t, "single line calls", ai.calls(), []call{{text: "hello"}})

	// Lines not kept are translated as one.
	it = cue(1, "hello", "world")
	ai, _ = translateItems(t, s, "es", perLine(func(string) string { return "hola mundo" }), it)
	check(t, "merged lines", it.translated, []string{"hola mundo"})
	check(t, "merged lines calls", ai.calls(), []call{{text: "hello world"}})

	// With formatting stripped, a long translation is wrapped.
	s.StripFormatting = true
	it = cue(1, "line a", "line b")
	if _, err := translateItems(t, s, "es", perLine(func(string) string { return longTranslation }), it); err != nil {
		t.Fatal(err)
	}
	if len(it.translated) < 2 {
		t.Errorf("wrapped = %q, want several lines", it.translated)
	}
	for _, l := range it.translated {
		if utf8.RuneCountInString(l) > maxLineLength {
			t.Errorf("wrapped line %q is longer than %d", l, maxLineLength)
		}
	}

	// Kept lines are translated one by one, verbatim.
	s = lingarrSettings()
	s.PreserveLineBreaks = true
	it = cue(1, "hello", "world")
	ai, _ = translateItems(t, s, "es", perLine(func(text string) string {
		if text == "hello" {
			return "hola"
		}
		return "mundo"
	}), it)
	check(t, "kept lines", it.translated, []string{"hola", "mundo"})
	check(t, "kept lines calls", ai.calls(), []call{{text: "hello"}, {text: "world"}})
	it = cue(1, "hello", "world")
	_, _ = translateItems(t, s, "es", perLine(func(text string) string {
		if text == "hello" {
			return "ho\nla"
		}
		return "mundo"
	}), it)
	check(t, "kept lines with a line break", it.translated, []string{"ho\nla", "mundo"})

	// A blank line is not translated.
	it = cue(1, "", "world")
	ai, _ = translateItems(t, s, "es", perLine(func(string) string { return "mundo" }), it)
	check(t, "blank line", it.translated, []string{"", "mundo"})
	check(t, "blank line calls", len(ai.calls()), 1)
}

func TestLingarrStackedCues(t *testing.T) {
	s := lingarrSettings()
	echo := perLine(func(text string) string { return "tr:" + text })
	texts := func(ai *scriptedAI) []string {
		var out []string
		for _, c := range ai.calls() {
			out = append(out, c.text)
		}
		return out
	}

	// The layers of a fansub sign are translated once.
	items := []*cueItem{timed(1, 1000, 2000, "Hello"), timed(2, 1000, 2000, "Hello"), timed(3, 1000, 2000, "Hello"), timed(4, 1000, 2000, "Hello")}
	ai, _ := translateItems(t, s, "es", echo, items...)
	check(t, "stacked calls", texts(ai), []string{"Hello"})
	for _, it := range items {
		check(t, "stacked translation", it.translated, []string{"tr:Hello"})
	}
	// Untimed cues of the same text are stacked too.
	items = []*cueItem{cue(1, "Yeah."), cue(2, "Yeah.")}
	ai, _ = translateItems(t, s, "es", echo, items...)
	check(t, "untimed stacked calls", texts(ai), []string{"Yeah."})
	check(t, "untimed stacked translation", items[1].translated, []string{"tr:Yeah."})

	for _, c := range []struct {
		name  string
		items []*cueItem
		want  []string
	}{
		{"same text at other times", []*cueItem{timed(1, 1000, 2000, "Yes"), timed(2, 5000, 6000, "Yes"), timed(3, 9000, 10000, "Yes")}, []string{"Yes", "Yes", "Yes"}},
		{"no duplicates", []*cueItem{timed(1, 1000, 2000, "Hello"), timed(2, 3000, 4000, "World"), timed(3, 5000, 6000, "How are you?")}, []string{"Hello", "World", "How are you?"}},
		{"same time, other text", []*cueItem{timed(1, 1000, 2000, "Run!"), timed(2, 1000, 2000, "Watch out!")}, []string{"Run!", "Watch out!"}},
	} {
		ai, _ := translateItems(t, s, "es", echo, c.items...)
		check(t, c.name, texts(ai), c.want)
		for _, it := range c.items {
			check(t, c.name+" translation", it.translated, []string{"tr:" + it.lines[0]})
		}
	}
}

func TestLingarrContext(t *testing.T) {
	echo := perLine(func(text string) string { return "tr:" + text })
	three := func() []*cueItem { return []*cueItem{cue(1, "Hello"), cue(2, "How are you"), cue(3, "Fine")} }

	s := lingarrSettings()
	s.ContextBefore, s.ContextAfter = 2, 1
	ai, _ := translateItems(t, s, "es", echo, three()...)
	check(t, "source context", ai.calls(), []call{
		{text: "Hello", after: []string{"How are you"}},
		{text: "How are you", before: []string{"Hello"}, after: []string{"Fine"}},
		{text: "Fine", before: []string{"Hello", "How are you"}},
	})

	// Translated context pairs the cues before with their translation.
	s.TranslatedContext, s.ContextAfter = true, 0
	ai, _ = translateItems(t, s, "es", echo, three()...)
	check(t, "translated context", ai.calls(), []call{
		{text: "Hello"},
		{text: "How are you", before: []string{contextJSON(1, "Hello", "tr:Hello")}},
		{text: "Fine", before: []string{contextJSON(1, "Hello", "tr:Hello"), contextJSON(2, "How are you", "tr:How are you")}},
	})
	check(t, "context line", contextJSON(1, "Hello", "tr:Hello"), `{"position":1,"line":"Hello","translation":"tr:Hello"}`)

	// The cues after are not translated yet.
	s.ContextBefore, s.ContextAfter = 1, 2
	ai, _ = translateItems(t, s, "es", echo, three()...)
	check(t, "translated context after", ai.calls(), []call{
		{text: "Hello", after: []string{contextJSON(2, "How are you", ""), contextJSON(3, "Fine", "")}},
		{text: "How are you", before: []string{contextJSON(1, "Hello", "tr:Hello")}, after: []string{contextJSON(3, "Fine", "")}},
		{text: "Fine", before: []string{contextJSON(2, "How are you", "tr:How are you")}},
	})
	check(t, "context line after", contextJSON(3, "Fine", ""), `{"position":3,"line":"Fine"}`)

	// A cue translated already, as by a translator that failed later in
	// its chunk, is not translated again and gives its translation.
	s.ContextAfter = 0
	done := cue(1, "Hello")
	done.translated = []string{"Hola"}
	ai, _ = translateItems(t, s, "es", echo, done, cue(2, "Fine"))
	check(t, "translated cue's context", ai.calls(), []call{{text: "Fine", before: []string{contextJSON(1, "Hello", "Hola")}}})

	// Kept lines are each one line of the context.
	s.PreserveLineBreaks = true
	first := cue(1, "Hello", "world")
	ai, _ = translateItems(t, s, "es", echo, first, cue(2, "Bye"))
	check(t, "kept lines translation", first.translated, []string{"tr:Hello", "tr:world"})
	check(t, "kept lines context", ai.calls()[2].before, []string{contextJSON(1, "Hello world", "tr:Hello tr:world")})

	// A cue the model answered nothing for keeps its text, given as not
	// translated.
	s.PreserveLineBreaks = false
	first = cue(1, "Hello")
	ai, _ = translateItems(t, s, "es", perLine(func(text string) string {
		if text == "Hello" {
			return ""
		}
		return "tr:" + text
	}), first, cue(2, "Bye"))
	check(t, "empty answer", first.translated, []string{"Hello"})
	check(t, "empty answer's context", ai.calls()[1].before, []string{contextJSON(1, "Hello", "")})

	// A translation's line breaks and other scripts stay readable on one
	// line of the context.
	ai, _ = translateItems(t, s, "es", perLine(func(text string) string {
		if text == "Hello world" {
			return "你好\n世界"
		}
		return "再见"
	}), cue(1, "Hello", "world"), cue(2, "Bye"))
	check(t, "context with a line break", ai.calls()[1].before, []string{`{"position":1,"line":"Hello world","translation":"你好\n世界"}`})
}

func TestLingarrBatch(t *testing.T) {
	s := lingarrSettings()
	s.Batch = true
	var seen []batchItem
	answer := func(line string) func(context.Context, *pluginv1.CompleteAiRequest) (string, error) {
		return batchAnswer(func(items []batchItem) []batchItem {
			seen = items
			out := make([]batchItem, len(items))
			for i, it := range items {
				out[i] = batchItem{Position: it.Position, Line: line}
			}
			return out
		})
	}

	it := cue(1, "hello", "world")
	if _, err := translateItems(t, s, "es", answer("hola mundo"), it); err != nil {
		t.Fatal(err)
	}
	check(t, "batch line", seen[0].Line, "hello world")
	check(t, "batch translation", it.translated, []string{"hola mundo"})

	s.PreserveLineBreaks = true
	it = cue(1, "hello", "world")
	_, _ = translateItems(t, s, "es", answer("hola\nmundo"), it)
	check(t, "kept lines batch line", seen[0].Line, "hello\nworld")
	check(t, "kept lines batch translation", it.translated, []string{"hola", "mundo"})
	it = cue(1, "hello", "world")
	_, _ = translateItems(t, s, "es", answer("hola\nmundo\nextra"), it)
	check(t, "more lines than the cue's", it.translated, []string{"hola mundo extra"})

	s.PreserveLineBreaks, s.StripFormatting = false, true
	it = cue(1, "line a", "line b")
	_, _ = translateItems(t, s, "es", answer(longTranslation), it)
	if len(it.translated) < 2 {
		t.Errorf("wrapped = %q, want several lines", it.translated)
	}

	s.StripFormatting = false
	it = cue(1, "hello", "world")
	_, _ = translateItems(t, s, "es", batchAnswer(func([]batchItem) []batchItem { return nil }), it)
	check(t, "cue missing", it.translated, []string{"hello", "world"})
}

func TestLingarrChain(t *testing.T) {
	e := newEnv(t)
	primary := &fakeTranslator{enabled: true, targets: []string{"zh"}}
	fallback := &fakeTranslator{enabled: true, targets: []string{"zh-TW"}}
	e.rt.Translators = func() []Translator {
		return []Translator{{ID: "primary", Client: primary}, {ID: "fallback", Client: fallback}}
	}
	ts := &e.s.Translation
	ts.Translator, ts.FallbackTranslators = "primary/upper", []string{"fallback/upper"}
	names := func(chain []chained) string {
		var out []string
		for _, c := range chain {
			out = append(out, c.name+">"+c.target)
		}
		return strings.Join(out, " ")
	}

	// An exact match comes before a parent of the language.
	chain, err := e.rt.chain(e.ctx, "", ts, "en", "zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	check(t, "chain", names(chain), "fallback/upper>zh-TW primary/upper>zh")
	// Of the same tier, the first of the settings comes first.
	primary.targets = []string{"zh-TW"}
	chain, _ = e.rt.chain(e.ctx, "", ts, "en", "zh-TW")
	check(t, "chain of the same tier", names(chain), "primary/upper>zh-TW fallback/upper>zh-TW")

	// The language matched is the one the translator is asked for.
	primary.targets, ts.FallbackTranslators = []string{"zh"}, nil
	en, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\nhi\n"), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rt.Translate(e.ctx, TranslatePayload{ItemID: e.movie.ID, Path: en, Language: core.SubtitleLanguage{Language: "zh-TW"}}); err != nil {
		t.Fatal(err)
	}
	check(t, "target asked", primary.requests[0].GetTargetLanguage(), "zh")

	primary.targets = []string{"zh-CN"}
	if _, err := e.rt.chain(e.ctx, "", ts, "en", "zh-TW"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("chain without a translator of the language: err = %v, want ErrUnavailable", err)
	}
}

func TestPortedGoogleGeminiService(t *testing.T) {
	portedCases(t, "google_gemini_service.json", ported{
		run: map[string]func(*testing.T, map[string]json.RawMessage){
			"TranslateAsync_ShouldRetry_WhenRetryableStatusCode":      func(t *testing.T, a map[string]json.RawMessage) { lingarrRetry(t, a, false, false) },
			"TranslateBatchAsync_ShouldRetry_WhenRetryableStatusCode": func(t *testing.T, a map[string]json.RawMessage) { lingarrRetry(t, a, true, false) },
			"TranslateAsync_ShouldThrow_WhenRetriesExhausted":         func(t *testing.T, a map[string]json.RawMessage) { lingarrRetry(t, a, false, true) },
			"TranslateBatchAsync_ShouldThrow_WhenRetriesExhausted":    func(t *testing.T, a map[string]json.RawMessage) { lingarrRetry(t, a, true, true) },
		},
		skip: map[string]string{
			"TranslateBatchAsync_ShouldRepairTruncatedJson_WhenResponseIsTruncated": "batches are answered through pi-ai's structured " +
				"output; a malformed answer is asked again rather than repaired (TestLingarrMalformedBatch)",
			"TranslateBatchAsync_ShouldRepairJson_WhenTruncatedInMiddleOfNumber":   "batches are answered through pi-ai's structured output; a malformed answer is asked again rather than repaired (TestLingarrMalformedBatch)",
			"TranslateBatchAsync_ShouldMergeDuplicatePositions_WhenJsonIsRepaired": "batches are answered through pi-ai's structured output; a malformed answer is asked again rather than repaired (TestLingarrMalformedBatch)",
			"TranslateBatchAsync_Integration_LargeBatch_ShouldHandleTruncation":    "calls Gemini over the network",
		},
		facts: map[string]string{
			"TranslateBatchAsync_ShouldMergeDuplicatePositions_WhenOneLineIsEmpty":                       "TestLingarrBatchPositions",
			"TranslateBatchAsync_ShouldMergeDuplicatePositions_WhenBothLinesHaveContent":                 "TestLingarrBatchPositions",
			"TranslateBatchAsync_ShouldThrowOperationCanceled_WhenCancelledDuringRequest":                "TestLingarrRequestEnd",
			"TranslateBatchAsync_ShouldThrowTranslationException_WhenRequestTimesOutWithoutCancellation": "TestLingarrRequestEnd",
			"TranslateAsync_ShouldReplacePlaceholders_AcrossTheWholeRequestTemplate":                     "TestLingarrPrompts",
			"TranslateBatchAsync_ShouldSendSerializedBatchAsUserMessage_AndEmptyContextPlaceholders":     "TestLingarrPrompts",
		},
	})
}

// lingarrRetry checks that a rate limit or an unavailable service is
// retried, until the retries are exhausted.
func lingarrRetry(t *testing.T, args map[string]json.RawMessage, batch, exhausted bool) {
	var status struct {
		Symbol string `json:"$symbol"`
	}
	if err := json.Unmarshal(args["statusCode"], &status); err != nil {
		t.Fatal(err)
	}
	code := map[string]connect.Code{
		"HttpStatusCode.TooManyRequests":    connect.CodeResourceExhausted,
		"HttpStatusCode.ServiceUnavailable": connect.CodeUnavailable,
	}[status.Symbol]
	s := lingarrSettings()
	s.Batch, s.MaxRetries = batch, 3
	attempts := 0
	ai, err := translateItems(t, s, "es", func(_ context.Context, req *pluginv1.CompleteAiRequest) (string, error) {
		attempts++
		if exhausted || attempts == 1 {
			return "", connect.NewError(code, errors.New(status.Symbol))
		}
		if batch {
			return `{"translations":[{"position":1,"line":"Hola"}]}`, nil
		}
		return "Hola", nil
	}, cue(1, "Hello"))
	if exhausted {
		if connect.CodeOf(err) != code || len(ai.requests) != 3 {
			t.Errorf("translation failing = %v after %d requests, want %v after 3", err, len(ai.requests), code)
		}
		return
	}
	if err != nil || len(ai.requests) != 2 {
		t.Errorf("translation = %v after %d requests, want success after 2", err, len(ai.requests))
	}
}

func TestLingarrBatchPositions(t *testing.T) {
	s := lingarrSettings()
	s.Batch = true
	for _, c := range []struct {
		answer string
		want   []string
	}{
		{`{"translations":[{"position":89,"line":""},{"position":89,"line":"Our love."}]}`, []string{"Our love."}},
		{`{"translations":[{"position":89,"line":"Our love."},{"position":89,"line":"It is beautiful."}]}`, []string{"Our love.\nIt is beautiful."}},
	} {
		it := cue(89, "Naše láska. Je krásná.")
		if _, err := translateItems(t, s, "en", func(context.Context, *pluginv1.CompleteAiRequest) (string, error) { return c.answer, nil }, it); err != nil {
			t.Fatal(err)
		}
		check(t, "translation of "+c.answer, it.translated, c.want)
	}
}

func TestLingarrRequestEnd(t *testing.T) {
	s := lingarrSettings()
	s.Batch = true
	ctx, cancel := context.WithCancel(t.Context())
	ai := &scriptedAI{answer: func(ctx context.Context, _ *pluginv1.CompleteAiRequest) (string, error) {
		cancel()
		<-ctx.Done()
		return "", ctx.Err()
	}}
	c := chained{translator: &translator{name: "ai/p/m", ai: ai, provider: "p", model: "m"}, source: "en", target: "es"}
	tr := &translation{
		r: &Runtime{}, s: &s, candidates: []chained{c}, logger: slog.New(slog.DiscardHandler),
		items: []*cueItem{cue(1, "Hello")}, record: &core.SubtitleTranslationRecord{},
	}
	if err := tr.run(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("translation canceled: err = %v, want context.Canceled", err)
	}

	// A request timing out fails the translation, not retried.
	s.RequestTimeout = 10 * time.Millisecond
	ai, err := translateItems(t, s, "es", func(ctx context.Context, _ *pluginv1.CompleteAiRequest) (string, error) {
		<-ctx.Done()
		return "", connect.NewError(connect.CodeDeadlineExceeded, ctx.Err())
	}, cue(1, "Hello"))
	if connect.CodeOf(err) != connect.CodeDeadlineExceeded || len(ai.requests) != 1 {
		t.Errorf("translation timing out = %v after %d requests, want deadline exceeded after 1", err, len(ai.requests))
	}
}

func TestLingarrPrompts(t *testing.T) {
	s := lingarrSettings()
	s.Prompt = "Translate from {sourceLanguage} to {targetLanguage}"
	s.UserPrompt = "Before: {contextBefore}\nLine: {lineToTranslate}\nAfter: {contextAfter}"
	s.ContextBefore, s.ContextAfter = 1, 1
	ai, err := translateItems(t, s, "nl", func(context.Context, *pluginv1.CompleteAiRequest) (string, error) { return "Hallo", nil },
		cue(1, "Line before"), cue(2, "Hello"), cue(3, "Line after"))
	if err != nil {
		t.Fatal(err)
	}
	check(t, "system prompt", ai.requests[1].GetSystem(), "Translate from English to Dutch")
	check(t, "user prompt", ai.requests[1].GetMessages()[0].GetText(), "Before: Line before\nLine: Hello\nAfter: Line after")

	// A batch goes as JSON, its prompt without context.
	s.Batch = true
	s.Prompt = "Translate from {sourceLanguage} to {targetLanguage}{contextBefore}{contextAfter}"
	ai, _ = translateItems(t, s, "es", batchAnswer(func(items []batchItem) []batchItem { return items }), cue(1, "Hello"))
	check(t, "batch system prompt", ai.requests[0].GetSystem(), "Translate from English to Spanish")
	check(t, "batch message", ai.requests[0].GetMessages()[0].GetText(), `[{"position":1,"line":"Hello"}]`)
}

func TestPortedLocalAiService(t *testing.T) {
	portedCases(t, "local_ai_service.json", ported{
		facts: map[string]string{
			"TranslateBatchAsync_ShouldRetry_WhenGenerateApiReturnsMalformedJson":        "TestLingarrMalformedBatch",
			"TranslateBatchAsync_ShouldThrow_WhenGenerateApiKeepsReturningMalformedJson": "TestLingarrMalformedBatch",
			"TranslateBatchAsync_ShouldNotRetry_WhenRequestFailsWithNonRetryableStatus":  "TestLingarrMalformedBatch",
		},
		skip: map[string]string{
			"TranslateBatchAsync_ShouldRetry_WhenChatApiFallbackReturnsMalformedJson": piAI,
		},
	})
}

func TestLingarrMalformedBatch(t *testing.T) {
	s := lingarrSettings()
	s.Batch, s.MaxRetries = true, 3
	answers := []string{"not json", `{"translations":[{"position":1,"line":"Hola"},{"position":2,"line":"Mundo"}]}`}
	ai, err := translateItems(t, s, "es", func(context.Context, *pluginv1.CompleteAiRequest) (string, error) {
		a := answers[0]
		answers = answers[1:]
		return a, nil
	}, cue(1, "Hello"), cue(2, "World"))
	if err != nil || len(ai.requests) != 2 {
		t.Errorf("translation = %v after %d requests, want success after 2", err, len(ai.requests))
	}

	ai, err = translateItems(t, s, "es", func(context.Context, *pluginv1.CompleteAiRequest) (string, error) { return "not json", nil }, cue(1, "Hello"))
	if err == nil || !strings.Contains(err.Error(), "the model's translations") || len(ai.requests) != 3 {
		t.Errorf("translation always malformed = %v after %d requests, want a failure after 3", err, len(ai.requests))
	}

	ai, err = translateItems(t, s, "es", func(context.Context, *pluginv1.CompleteAiRequest) (string, error) {
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("bad request"))
	}, cue(1, "Hello"))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || len(ai.requests) != 1 {
		t.Errorf("translation refused = %v after %d requests, want failed precondition after 1", err, len(ai.requests))
	}
}

func TestPortedTranslateController(t *testing.T) {
	for _, name := range []string{"translate_controller_content.json", "translate_controller_line.json"} {
		var f caseFile
		data, err := os.ReadFile(path.Join("testdata", "cases", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatal(err)
		}
		skip := map[string]string{}
		for _, m := range f.Facts {
			skip[m.Method] = noContentAPI
		}
		portedCases(t, name, ported{skip: skip})
	}
}

func TestPortedLanguageCodeService(t *testing.T) {
	str := func(t *testing.T, a map[string]json.RawMessage, key string) string {
		t.Helper()
		var v string
		if err := json.Unmarshal(a[key], &v); err != nil && string(a[key]) != "null" {
			t.Fatal(err)
		}
		return v
	}
	valid := func(want bool) func(*testing.T, map[string]json.RawMessage) {
		return func(t *testing.T, a map[string]json.RawMessage) {
			code := str(t, a, "code")
			if got := Tag(code) != ""; got != want {
				t.Errorf("Tag(%q) valid = %v, want = %v", code, got, want)
			}
		}
	}
	const (
		names = "Mavio names languages in prompts with CLDR's English names, such as Brazilian Portuguese, not .NET's culture names"
		cased = "Mavio records languages as BCP 47 tags in their canonical case (pt-BR), compared case-insensitively"
		list  = "Mavio takes any BCP 47 tag; translators report the languages they translate"
	)
	portedCases(t, "language_code_service.json", ported{
		run: map[string]func(*testing.T, map[string]json.RawMessage){
			"Validate_WithTwoLetterIsoCodes_ReturnsTrue":  valid(true),
			"Validate_WithRegionCodes_ReturnsTrue":        valid(true),
			"Validate_WithLegacyChineseCodes_ReturnsTrue": valid(true),
			"Validate_WithInvalidCodes_ReturnsFalse":      valid(false),
			"GetCultureName_WithTwoLetterCodes_ReturnsCorrectName": func(t *testing.T, a map[string]json.RawMessage) {
				code, want := str(t, a, "code"), str(t, a, "expectedName")
				check(t, "languageName("+code+")", languageName(code, false), want)
			},
			"SubtitleFilenameDetection_ExtractsCorrectLanguageCode": func(t *testing.T, a map[string]json.RawMessage) {
				name, want := str(t, a, "filename"), str(t, a, "expectedCode")
				stem := strings.TrimSuffix(name, path.Ext(name))
				got := Tag(stem[strings.LastIndex(stem, ".")+1:])
				if !strings.EqualFold(got, want) {
					t.Errorf("language of %s = %q, want = %q", name, got, want)
				}
			},
		},
		skip: map[string]string{
			"GetCultureName_WithRegionCodes_ReturnsCorrectName":                  names,
			"GetCultureName_WithLegacyChineseCodes_ReturnsCorrectName":           names,
			"GetCultureName_WithInvalidCode_ThrowsArgumentException":             "prompts name a language Mavio cannot name by its code rather than failing",
			"GetNormalizedCode_WithTwoLetterCodes_ReturnsLowercase":              cased,
			"GetNormalizedCode_WithRegionCodes_ReturnsLowercasePreservingRegion": cased,
			"GetNormalizedCode_WithLegacyChineseCodes_PreservesLegacyFormat":     cased,
			"GetNormalizedCode_WithInvalidCode_ThrowsArgumentException":          cased,
			"GetSupportedLanguages_IncludesNeutralsAndAllowedSpecifics":          list,
			"GetSupportedLanguages_ExcludesUnlistedRegionalVariants":             list,
			"GetSupportedLanguages_AllEntriesHaveEmptyTargets":                   list,
		},
		facts: map[string]string{
			"GetBestMatch_ExactBeatsNeutral":                      "TestLingarrBestMatch",
			"GetBestMatch_AliasEquivalentBeatsNeutral":            "TestLingarrBestMatch",
			"GetBestMatch_ScriptEquivalentBeatsNeutral":           "TestLingarrBestMatch",
			"GetBestMatch_NeutralFallbackWhenNothingMoreSpecific": "TestLingarrBestMatch",
			"GetBestMatch_DifferentScriptDoesNotMatch":            "TestLingarrBestMatch",
			"GetBestMatch_RegionSiblingDoesNotMatch":              "TestLingarrBestMatch",
			"GetBestMatch_NeutralBeatsSibling":                    "TestLingarrBestMatch",
			"GetBestMatch_CaseInsensitiveExact":                   "TestLingarrBestMatch",
			"GetBestMatch_NullOrUnknownRequestedReturnsNull":      "TestLingarrBestMatch",
		},
	})
}

func TestLingarrBestMatch(t *testing.T) {
	for _, c := range []struct {
		want      string
		languages []string
		match     string
		tier      matchTier
	}{
		{"zh-TW", []string{"zh", "zh-CN", "zh-TW"}, "zh-TW", matchExact},
		{"zh-TW", []string{"zh", "zh-Hant-TW"}, "zh-Hant-TW", matchAlias},
		{"zh-TW", []string{"zh-Hans", "zh-Hant"}, "zh-Hant", matchScript},
		{"zh-TW", []string{"zh", "zh-CN"}, "zh", matchNeutral},
		{"zh-TW", []string{"zh-CN"}, "", matchNone},
		{"pt-BR", []string{"pt-PT"}, "", matchNone},
		{"pt-PT", []string{"pt-BR"}, "", matchNone},
		{"pt-BR", []string{"pt-PT", "pt"}, "pt", matchNeutral},
		{"ZH-tw", []string{"zh-TW"}, "zh-TW", matchExact},
		{"", []string{"en"}, "", matchNone},
		{"notacode", []string{"en"}, "", matchNone},
	} {
		if m, tier := matchLanguage(c.want, c.languages); m != c.match || tier != c.tier {
			t.Errorf("matchLanguage(%q, %v) = %q, %d; want = %q, %d", c.want, c.languages, m, tier, c.match, c.tier)
		}
	}
	if _, err := language.Parse("zh-TW"); err != nil {
		t.Fatal(err)
	}
}

// automation is a movie of the automation's tests: subtitle files in
// the languages given, as "en", "en.hi" or "en.forced", and a profile
// wanting the targets.
func automation(t *testing.T, files []string, sources []string, targets ...string) *env {
	t.Helper()
	e := newEnv(t)
	// Items are added at the clock's time, after the test's.
	e.movie.DateAdded = e.rt.now().Add(-1000 * time.Hour)
	if err := e.db.Items().Upsert(e.ctx, e.movie); err != nil {
		t.Fatal(err)
	}
	streams := []core.MediaStream{
		{Index: 0, Kind: core.StreamVideo, Codec: "h264"},
		{Index: 1, Kind: core.StreamAudio, Codec: "aac", Language: "eng"},
	}
	for i, f := range files {
		lang, flag, _ := strings.Cut(f, ".")
		ext := "srt"
		if l, x, ok := strings.Cut(lang, ":"); ok {
			lang, ext = l, x
		}
		p := path.Join(e.dir, fmt.Sprintf("The Matrix (1999).%s.%s", strings.ReplaceAll(f, ":"+ext, ""), ext))
		if err := os.WriteFile(p, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		streams = append(streams, core.MediaStream{
			Index: 2 + i, Kind: core.StreamSubtitle, Codec: ext, Language: lang, ExternalPath: p,
			Forced: flag == "forced", HearingImpaired: flag == "hi" || flag == "sdh" || flag == "cc",
		})
	}
	src := core.MediaSource{ID: core.NewID(), ItemID: e.movie.ID, Path: e.movie.Path, Size: 128 << 10, Container: "matroska", Streams: streams}
	if err := e.db.MediaSources().Replace(e.ctx, e.movie.ID, []core.MediaSource{src}); err != nil {
		t.Fatal(err)
	}
	p := core.SubtitleProfile{ID: core.NewID(), Name: "Targets"}
	for i, l := range targets {
		p.Items = append(p.Items, core.SubtitleProfileItem{ID: i + 1, Language: l, HI: core.SubtitleHIAllowed})
	}
	if err := e.db.SubtitleProfiles().Put(e.ctx, &p); err != nil {
		t.Fatal(err)
	}
	if err := e.db.SubtitleProfileAssignments().Put(e.ctx, core.SubtitleProfileAssignment{TargetID: e.lib.ID, ProfileID: p.ID}); err != nil {
		t.Fatal(err)
	}
	a := &e.s.Translation.Automation
	a.Enabled, a.SourceLanguages, a.MovieAge = true, sources, 0
	return e
}

// queue runs the automation for the movie and returns the translations
// pending, by target language, with the file each translates.
func (e *env) queue() map[string]string {
	e.t.Helper()
	running, err := e.rt.runningTranslations(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.rt.translateMissing(e.ctx, e.movie.ID, &e.s, e.rt.now(), running); err != nil {
		e.t.Fatal(err)
	}
	page, err := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobTranslate}, States: []core.JobState{core.JobPending}})
	if err != nil {
		e.t.Fatal(err)
	}
	out := map[string]string{}
	for _, j := range page.Items {
		var p TranslatePayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			e.t.Fatal(err)
		}
		out[p.Language.String()] = path.Base(p.Path)
	}
	return out
}

// translation queues a translation of the movie into a language.
func (e *env) translation(lang string) core.Job {
	e.t.Helper()
	l := core.SubtitleLanguage{Language: lang}
	payload, _ := json.Marshal(TranslatePayload{ItemID: e.movie.ID, Language: l})
	j := core.Job{
		ID: core.NewID(), Kind: JobTranslate, Payload: payload, MaxAttempts: 1, RunAt: e.rt.now(),
		UniqueKey: fmt.Sprintf("%s:%s:%s", JobTranslate, e.movie.ID, l),
	}
	if _, err := e.db.Jobs().Enqueue(e.ctx, &j); err != nil {
		e.t.Fatal(err)
	}
	return j
}

func (e *env) lease() core.Job {
	e.t.Helper()
	j, err := e.db.Jobs().Lease(e.ctx, "test", []string{JobTranslate}, time.Hour)
	if err != nil {
		e.t.Fatal(err)
	}
	return j
}

const one = "The Matrix (1999).en.srt"

func TestPortedLingarrMediaSubtitleProcessor(t *testing.T) {
	const captions = "Mavio never translates from forced subtitles, which hold only the foreign parts, and prefers others to those " +
		"for the hearing impaired, as Lingarr with captions ignored; it has no option to take the first"
	for name, facts := range map[string]map[string]string{
		"caption_handling.json": {
			"ProcessMedia_WithForcedSubtitleOnly_ShouldProcessWhenIgnoreCaptionsTrue":                  "TestLingarrCaptions",
			"ProcessMedia_WithBothForcedAndRegularSubtitles_ShouldPreferRegularWhenIgnoreCaptionsTrue": "TestLingarrCaptions",
			"ProcessMedia_WithHiSubtitleOnly_ShouldProcessWhenIgnoreCaptionsTrue":                      "TestLingarrCaptions",
			"ProcessMedia_WithSdhSubtitle_ShouldProcessWhenIgnoreCaptionsTrue":                         "TestLingarrCaptions",
			"ProcessMedia_WithCcSubtitle_ShouldProcessWhenIgnoreCaptionsTrue":                          "TestLingarrCaptions",
			"ProcessMedia_WithAllCaptionTypes_ShouldPreferNonCaption":                                  "TestLingarrCaptions",
		},
		"duplicate_translation.json": {
			"ProcessMedia_WithPendingRequest_ShouldNotCreateDuplicate":       "TestLingarrDuplicateTranslation",
			"ProcessMedia_WithInProgressRequest_ShouldNotCreateDuplicate":    "TestLingarrDuplicateTranslation",
			"ProcessMedia_WithCompletedRequest_ShouldNotCreateDuplicate":     "TestLingarrDuplicateTranslation",
			"ProcessMedia_WithFailedRequest_ShouldCreateNewRequest":          "TestLingarrDuplicateTranslation",
			"ProcessMedia_PartialDuplicate_ShouldOnlyCreateMissingLanguages": "TestLingarrDuplicateTranslation",
		},
		"language_handling.json": {
			"ProcessMedia_AllTargetLanguagesExist_ShouldReturnFalseAndNotCreateRequest": "TestLingarrLanguages",
			"ProcessMedia_WithHindiLanguage_ShouldProcessCorrectly":                     "TestLingarrLanguages",
		},
		"subtitle_format.json": {
			"ProcessMedia_WithAssFormat_ShouldProcess":          "TestLingarrSourceFormats",
			"ProcessMedia_WithSsaFormat_ShouldProcess":          "TestLingarrSourceFormats",
			"ProcessMedia_WithMultipleFormats_ShouldProcessAll": "TestLingarrSourceFormats",
		},
	} {
		portedCases(t, name, ported{facts: facts, skip: map[string]string{
			"ProcessMedia_WithForcedSubtitle_ShouldAlwaysProcessWhenIgnoreCaptionsFalse": captions,
			"ProcessMedia_WithCancelledRequest_ShouldCreateNewRequest": "Mavio's jobs end in success or failure; " +
				"a failed translation is queued again (ProcessMedia_WithFailedRequest_ShouldCreateNewRequest)",
		}})
	}
}

func TestLingarrCaptions(t *testing.T) {
	for _, c := range []struct {
		name    string
		files   []string
		targets []string
		want    map[string]string
	}{
		{"forced only, target present", []string{"en.forced", "ro"}, []string{"ro"}, map[string]string{}},
		{"forced and regular", []string{"en.forced", "en"}, []string{"ro"}, map[string]string{"ro": one}},
		{"hi only", []string{"en.hi"}, []string{"ro"}, map[string]string{"ro": "The Matrix (1999).en.hi.srt"}},
		{"sdh only", []string{"en.sdh"}, []string{"ro"}, map[string]string{"ro": "The Matrix (1999).en.sdh.srt"}},
		{"cc only", []string{"en.cc"}, []string{"ro"}, map[string]string{"ro": "The Matrix (1999).en.cc.srt"}},
		{"all captions", []string{"en.forced", "en.hi", "en.sdh", "en.cc", "en"}, []string{"ro"}, map[string]string{"ro": one}},
	} {
		e := automation(t, c.files, []string{"en"}, c.targets...)
		check(t, c.name, e.queue(), c.want)
	}
}

func TestLingarrDuplicateTranslation(t *testing.T) {
	// Pending.
	e := automation(t, []string{"en"}, []string{"en"}, "ro")
	e.translation("ro")
	e.queue()
	if page, _ := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobTranslate}}); page.Total != 1 {
		t.Errorf("translations with one pending = %d, want 1", page.Total)
	}
	// Running.
	e = automation(t, []string{"en"}, []string{"en"}, "ro")
	e.translation("ro")
	e.lease()
	check(t, "with one running", e.queue(), map[string]string{})
	// Done: the translation is a subtitle of the language.
	e = automation(t, []string{"en", "ro"}, []string{"en"}, "ro")
	check(t, "with one done", e.queue(), map[string]string{})
	// Failed.
	e = automation(t, []string{"en"}, []string{"en"}, "ro")
	e.translation("ro")
	if err := e.db.Jobs().Fail(e.ctx, e.lease().ID, "test", errors.New("broken")); err != nil {
		t.Fatal(err)
	}
	check(t, "with one failed", e.queue(), map[string]string{"ro": one})
	// One of two pending.
	e = automation(t, []string{"en"}, []string{"en"}, "fr", "de")
	e.translation("fr")
	got := e.queue()
	check(t, "with one of two pending", got["de"], one)
	if page, _ := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobTranslate}}); page.Total != 2 {
		t.Errorf("translations = %d, want 2", page.Total)
	}
}

func TestLingarrLanguages(t *testing.T) {
	e := automation(t, []string{"en", "ro"}, []string{"en"}, "ro")
	check(t, "all present", e.queue(), map[string]string{})
	e = automation(t, []string{"hi"}, []string{"hi"}, "en")
	check(t, "Hindi", e.queue(), map[string]string{"en": "The Matrix (1999).hi.srt"})
}

func TestLingarrSourceFormats(t *testing.T) {
	for _, c := range []struct {
		files []string
		want  string
	}{
		{[]string{"en:ass"}, "The Matrix (1999).en.ass"},
		{[]string{"en:ssa"}, "The Matrix (1999).en.ssa"},
		{[]string{"en", "en:ass", "en:ssa"}, one},
	} {
		e := automation(t, c.files, []string{"en"}, "ro")
		check(t, fmt.Sprint(c.files), e.queue(), map[string]string{"ro": c.want})
	}
}

func TestPortedAutomatedTranslationJob(t *testing.T) {
	portedCases(t, "automated_translation_job.json", ported{facts: map[string]string{
		"ProcessMovies_ScansPastInitialWindowWhenEarlyItemsTooNew": "TestLingarrAutomationAge",
	}})
}

func TestLingarrAutomationAge(t *testing.T) {
	e := automation(t, []string{"en"}, []string{"en"}, "ro")
	a := &e.s.Translation.Automation
	a.MaxPerRun, a.MovieAge = 1, 48*time.Hour
	now := e.rt.now()
	// Two movies too new come before the movie old enough.
	for i, age := range []time.Duration{time.Hour, 2 * time.Hour} {
		m := e.movie
		m.ID, m.Name, m.DateAdded = core.NewID(), fmt.Sprintf("Too Fresh %d", i+1), now.Add(-age)
		m.Path = path.Join(e.dir, m.Name+".mkv")
		if err := e.db.Items().Upsert(e.ctx, m); err != nil {
			t.Fatal(err)
		}
		src := core.MediaSource{ID: core.NewID(), ItemID: m.ID, Path: m.Path, Size: 1, Container: "matroska", Streams: []core.MediaStream{
			{Index: 0, Kind: core.StreamSubtitle, Codec: "srt", Language: "en", ExternalPath: path.Join(e.dir, one)},
		}}
		if err := e.db.MediaSources().Replace(e.ctx, m.ID, []core.MediaSource{src}); err != nil {
			t.Fatal(err)
		}
		if err := e.db.SubtitleWanted().Put(e.ctx, core.SubtitleWanted{
			ItemID: m.ID, Kind: m.Kind, LibraryID: e.lib.ID, Path: m.Path, Missing: []core.SubtitleLanguage{{Language: "ro"}}, Updated: now.Add(-time.Duration(i+2) * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	e.movie.DateAdded = now.Add(-80 * time.Hour)
	if err := e.db.Items().Upsert(e.ctx, e.movie); err != nil {
		t.Fatal(err)
	}
	if err := e.db.SubtitleWanted().Put(e.ctx, core.SubtitleWanted{
		ItemID: e.movie.ID, Kind: e.movie.Kind, LibraryID: e.lib.ID, Path: e.movie.Path, Missing: []core.SubtitleLanguage{{Language: "ro"}}, Updated: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.rt.TranslateWanted(e.ctx); err != nil {
		t.Fatal(err)
	}
	page, err := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobTranslate}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || !strings.Contains(string(page.Items[0].Payload), e.movie.ID.String()) {
		t.Errorf("translations = %+v, want one of the movie old enough", page.Items)
	}
}
