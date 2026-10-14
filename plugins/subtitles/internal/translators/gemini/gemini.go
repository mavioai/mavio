// Package gemini is the translator of Google's Gemini models, ported from
// Bazarr's GeminiTranslatorService: cues go in batches, as JSON, with an
// instruction describing the movie or series; batches failing are split,
// and keys rate limited rest while others are used.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/bidi"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// Name is the translator's name.
const Name = "gemini"

const (
	defaultModel     = "gemini-2.0-flash"
	defaultBatchSize = 300
	// defaultCooldown is how long a key rate limited rests when the
	// answer does not say.
	defaultCooldown = 60 * time.Second
	// retries is how many times a batch failing is tried again, split.
	retries = 3
)

// Definition returns the translator's definition.
func Definition() translator.Definition {
	return translator.Definition{
		Name: Name, DisplayName: "Gemini", Markup: true,
		Settings: []provider.Setting{
			{Name: "api_keys", Label: "API keys", Help: "Gemini API keys, separated by commas; keys rate limited rest while the others are used.", Type: provider.Password},
			{Name: "model", Label: "Model", Type: provider.Text, Default: defaultModel},
			{Name: "batch_size", Label: "Batch size", Help: "How many cues are sent at once.", Type: provider.Number, Default: defaultBatchSize, Advanced: true},
		},
		New: New,
	}
}

type settings struct {
	APIKeys   keys   `json:"api_keys"`
	Model     string `json:"model"`
	BatchSize *int   `json:"batch_size"`
}

// keys are API keys, as a list or a string separated by commas.
type keys []string

func (k *keys) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		list = strings.Split(s, ",")
	}
	*k = nil
	for _, key := range list {
		if key = strings.TrimSpace(key); key != "" && !slices.Contains(*k, key) {
			*k = append(*k, key)
		}
	}
	return nil
}

// New makes the translator from its settings.
func New(env provider.Env, raw json.RawMessage) (translator.Translator, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if len(s.APIKeys) == 0 {
		return nil, provider.Errorf(provider.ConfigurationError, "please provide at least one valid Gemini API key")
	}
	if s.Model = strings.TrimSpace(s.Model); s.Model == "" {
		s.Model = defaultModel
	}
	batch := defaultBatchSize
	if s.BatchSize != nil {
		batch = max(1, *s.BatchSize)
	}
	return &Translator{
		env: env, keys: s.APIKeys, model: s.Model, batch: batch, current: -1,
		api: "https://generativelanguage.googleapis.com/v1beta", cooldowns: map[string]time.Time{},
	}, nil
}

// Translator is Gemini.
type Translator struct {
	env   provider.Env
	keys  []string
	model string
	batch int
	api   string

	mu sync.Mutex
	// current is the index of the key in use; -1 before the first.
	current   int
	cooldowns map[string]time.Time
}

// SetBaseURL points the translator at another server, for tests.
func (t *Translator) SetBaseURL(u string) { t.api = strings.TrimSuffix(u, "/") }

// Credit implements translator.Translator: the model's name.
func (t *Translator) Credit() string { return t.model }

// BatchSize implements translator.Translator.
func (t *Translator) BatchSize() int { return t.batch }

// item is a cue as Gemini takes it.
type item struct {
	Index   string `json:"index"`
	Content string `json:"content"`
}

// Translate implements translator.Translator.
func (t *Translator) Translate(ctx context.Context, req *translator.Request) ([]string, error) {
	out := make([]string, len(req.Lines))
	batch := make([]item, len(req.Lines))
	for i, l := range req.Lines {
		batch[i] = item{Index: strconv.Itoa(i), Content: l}
	}
	if len(batch) == 0 {
		return out, nil
	}
	key := t.key()
	if key == "" {
		return nil, provider.Errorf(provider.TooManyRequests, "all Gemini API keys are currently rate limited, please wait before retrying")
	}
	instruction := Instruction(translator.Name(req.Target), Description(req.Media))
	if err := t.process(ctx, instruction, batch, out, retries); err != nil {
		return nil, err
	}
	return out, nil
}

// Instruction is the instruction translating to a language, as Bazarr's
// get_instruction.
func Instruction(language, description string) string {
	s := `You are an assistant that translates subtitles to ` + language + `.
    You will receive the following JSON type:

    class SubtitleObject(typing.TypedDict):
        index: str
        content: str

    Request: list[SubtitleObject]

    The 'index' key is the index of the subtitle dialog.
    The 'content' key is the dialog to be translated.

    The indices must remain the same in the response as in the request.
    Dialogs must be translated as they are without any changes.
    If a line has a comma or multiple sentences, try to keep one line to about 40-50 characters.
    `
	if description != "" {
		s += "\nAdditional user instruction: '" + description + "'"
	}
	return s
}

// Description describes a movie or series, as Bazarr's get_description;
// "" without a title.
func Description(m translator.Media) string {
	if m.Title == "" {
		return ""
	}
	kind := "movie"
	if m.Episode {
		kind = "TV show"
	}
	year := ""
	if m.Year > 0 {
		year = strconv.Itoa(m.Year)
	}
	return fmt.Sprintf("You will translate %s that is called %s from %s and it has IMDB ID = %s. Its overview: %s",
		kind, m.Title, year, m.ExternalIDs["imdb"], m.Overview)
}

// key returns the key in use, or the next one resting no longer; "" when
// all rest.
func (t *Translator) key() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current >= 0 && !t.resting(t.keys[t.current]) {
		return t.keys[t.current]
	}
	return t.rotate()
}

// rotate picks the next key resting no longer, as Bazarr's
// _select_next_api_key.
func (t *Translator) rotate() string {
	for off := 1; off <= len(t.keys); off++ {
		i := (max(t.current, -1) + off + len(t.keys)) % len(t.keys)
		if !t.resting(t.keys[i]) {
			t.current = i
			return t.keys[i]
		}
	}
	return ""
}

func (t *Translator) resting(key string) bool {
	until, ok := t.cooldowns[key]
	if !ok {
		return false
	}
	if !t.env.Now().Before(until) {
		delete(t.cooldowns, key)
		return false
	}
	return true
}

// rest rests the key in use for d and rotates to another; false when all
// rest.
func (t *Translator) rest(key string, d time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cooldowns[key] = t.env.Now().Add(max(time.Second, d))
	return t.rotate() != ""
}

// apiError is an answer failing.
type apiError struct {
	status     int
	retryAfter string
	body       []byte
}

func (e *apiError) Error() string {
	return fmt.Sprintf("gemini answered %d: %s", e.status, bytes.TrimSpace(e.body))
}

// rateLimited reports whether an answer failing is a rate limit, as
// Bazarr's _is_rate_limited_response.
func (e *apiError) rateLimited() bool {
	if e.status == http.StatusTooManyRequests {
		return true
	}
	var body struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(e.body, &body) != nil {
		return false
	}
	msg := strings.ToLower(body.Error.Message)
	return strings.ToUpper(body.Error.Status) == "RESOURCE_EXHAUSTED" || strings.Contains(msg, "rate limit") || strings.Contains(msg, "quota")
}

// cooldown is how long the answer says to wait, as Bazarr's
// _get_retry_after_seconds.
func (e *apiError) cooldown() time.Duration {
	if f, err := strconv.ParseFloat(e.retryAfter, 64); err == nil {
		return max(time.Second, time.Duration(int(f))*time.Second)
	}
	return defaultCooldown
}

// process translates a batch into out, as Bazarr's _process_batch: a rate
// limit rotates the key and tries again; other failures split the batch
// and try its halves.
func (t *Translator) process(ctx context.Context, instruction string, batch []item, out []string, retry int) error {
	err := t.request(ctx, instruction, batch, out)
	if err == nil || ctx.Err() != nil {
		return err
	}
	if ae, ok := errors.AsType[*apiError](err); ok && ae.rateLimited() {
		key := t.key()
		if !t.rest(key, ae.cooldown()) {
			return provider.Errorf(provider.TooManyRequests, "all Gemini API keys are currently rate limited, please wait before retrying")
		}
		t.env.Log.WarnContext(ctx, "gemini key rate limited, rotating to another configured key")
		if retry > 0 {
			return t.process(ctx, instruction, batch, out, retry-1)
		}
		return provider.Errorf(provider.TooManyRequests, "translation request failed: %w", err)
	}
	if len(batch) > 1 && retry > 0 {
		mid := len(batch) / 2
		if err := t.process(ctx, instruction, batch[:mid], out, retry-1); err != nil {
			return err
		}
		return t.process(ctx, instruction, batch[mid:], out, retry-1)
	}
	return fmt.Errorf("translation request failed: %w", err)
}

// fence is the Markdown fence Gemini may wrap its JSON in.
var fence = regexp.MustCompile("(?s)^```json\\s*(.*?)\\s*```$")

// request translates a batch into out.
func (t *Translator) request(ctx context.Context, instruction string, batch []item, out []string) error {
	contents, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role,omitempty"`
		Parts []part `json:"parts"`
	}
	payload, err := json.Marshal(struct {
		SystemInstruction content   `json:"system_instruction"`
		Contents          []content `json:"contents"`
	}{
		SystemInstruction: content{Parts: []part{{Text: instruction}}},
		Contents:          []content{{Role: "user", Parts: []part{{Text: string(contents)}}}},
	})
	if err != nil {
		return err
	}
	u := t.api + "/models/" + url.PathEscape(t.model) + ":generateContent?key=" + url.QueryEscape(t.key())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.env.HTTP.Do(req)
	if err != nil {
		// The URL holds the key.
		var ue *url.Error
		if errors.As(err, &ue) {
			return ue.Err
		}
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &apiError{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After"), body: body}
	}
	var answer struct {
		Candidates []struct {
			Content struct {
				Parts []part `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return fmt.Errorf("parse answer: %w", err)
	}
	if len(answer.Candidates) == 0 {
		return errors.New("gemini answered no candidate")
	}
	var text strings.Builder
	for _, p := range answer.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	result := strings.TrimSpace(fence.ReplaceAllString(text.String(), "$1"))
	var lines []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &lines); err != nil {
		return fmt.Errorf("parse translation: %w", err)
	}
	if len(lines) != len(batch) {
		return fmt.Errorf("gemini returned %d lines instead of expected %d lines", len(lines), len(batch))
	}
	want := map[string]bool{}
	for _, it := range batch {
		want[it.Index] = true
	}
	got := make([]string, len(out))
	copy(got, out)
	for _, l := range lines {
		rawIndex, ok1 := l["index"]
		rawContent, ok2 := l["content"]
		if !ok1 || !ok2 {
			break
		}
		index, content := jsonString(rawIndex), ""
		if err := json.Unmarshal(rawContent, &content); err != nil {
			return fmt.Errorf("parse translation: %w", err)
		}
		if !want[index] {
			return errors.New("gemini has returned different indices")
		}
		i, _ := strconv.Atoi(index)
		if rtl(content) {
			content = "\u202b" + content + "\u202c"
		}
		got[i] = content
	}
	copy(out, got)
	return nil
}

// jsonString returns a JSON string's value, or a number's text.
func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// rtl reports whether right-to-left characters outnumber left-to-right
// ones, as Bazarr's _dominant_strong_direction.
func rtl(s string) bool {
	var r, l int
	for _, c := range s {
		p, _ := bidi.LookupRune(c)
		switch p.Class() {
		case bidi.R, bidi.AL, bidi.RLE, bidi.RLI:
			r++
		case bidi.L, bidi.LRE, bidi.LRI:
			l++
		}
	}
	return r > l
}
