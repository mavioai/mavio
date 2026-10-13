package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fakeProvider answers searches with its subtitles or its failure.
type fakeProvider struct {
	mu    sync.Mutex
	subs  []provider.Subtitle
	err   error
	langs [][]provider.Language
}

func (f *fakeProvider) Search(_ context.Context, _ *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.langs = append(f.langs, langs)
	return f.subs, f.err
}

func (f *fakeProvider) Download(_ context.Context, v *provider.Video, id string) (provider.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return provider.File{}, f.err
	}
	return provider.File{Data: []byte(id + " " + v.Title), Format: "srt", Language: provider.Language{Tag: "en"}}, nil
}

type harness struct {
	p     *plugin
	now   time.Time
	fakes map[string]*fakeProvider
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	p, err := newPlugin(slog.New(slog.DiscardHandler), "1", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{p: p, now: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), fakes: map[string]*fakeProvider{}}
	p.now = func() time.Time { return h.now }
	p.defs = nil
	clear(p.byName)
	for _, name := range []string{"a", "b", "fr-only", "movies"} {
		f := &fakeProvider{}
		h.fakes[name] = f
		def := provider.Definition{
			Name: name, DisplayName: name, Movies: true, Episodes: name != "movies", HashVerifiable: name == "a",
			Settings: []provider.Setting{{Name: "key", Label: "Key", Type: provider.Password}, {Name: "n", Type: provider.Number, Default: 3}},
			Throttle: map[provider.Kind]func(time.Time) time.Duration{provider.DownloadLimitExceeded: provider.Fixed(6 * time.Hour)},
			New: func(_ provider.Env, s json.RawMessage) (provider.Provider, error) {
				var settings struct{ Key string }
				if err := json.Unmarshal(s, &settings); err != nil || settings.Key == "" {
					return nil, provider.Errorf(provider.ConfigurationError, "no key")
				}
				return f, nil
			},
		}
		if name == "fr-only" {
			def.Languages = []string{"fr"}
		}
		p.defs = append(p.defs, def)
	}
	for i := range p.defs {
		p.byName[p.defs[i].Name] = &p.defs[i]
	}
	return h
}

func (h *harness) configure(t *testing.T, cfg string) {
	t.Helper()
	if _, err := h.p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(cfg)}.Build()); err != nil {
		t.Fatal(err)
	}
}

func (h *harness) search(t *testing.T, kind pluginv1.MediaKind, langs ...string) *pluginv1.SearchSubtitlesResponse {
	t.Helper()
	req := pluginv1.SearchSubtitlesRequest_builder{Video: pluginv1.SubtitleVideo_builder{Kind: kind.Enum(), Title: proto.String("Up")}.Build()}.Build()
	for _, l := range langs {
		req.SetLanguages(append(req.GetLanguages(), language(provider.Language{Tag: l})))
	}
	resp, err := h.p.SearchSubtitles(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

const allEnabled = `{"providers":[
	{"name":"a","enabled":true,"settings":{"key":"k"}},
	{"name":"b","enabled":true,"settings":{"key":"k"}},
	{"name":"fr-only","enabled":true,"settings":{"key":"k"}},
	{"name":"movies","enabled":true,"settings":{"key":"k"}}]}`

func providers(resp *pluginv1.SearchSubtitlesResponse) []string {
	var out []string
	for _, s := range resp.GetSubtitles() {
		out = append(out, s.GetProvider()+"/"+s.GetId())
	}
	slices.Sort(out)
	return out
}

func TestSearch(t *testing.T) {
	h := newHarness(t)
	h.configure(t, allEnabled)
	for name, f := range h.fakes {
		f.subs = []provider.Subtitle{{ID: "1", Language: provider.Language{Tag: "en"}, Matches: []string{"title"}, Downloads: 3}}
		if name == "b" {
			f.subs[0].HashVerifiable = new(true)
		}
	}
	resp := h.search(t, pluginv1.MediaKind_MEDIA_KIND_EPISODE, "en")
	// fr-only has no English subtitles and movies no episodes.
	if got, want := providers(resp), []string{"a/1", "b/1"}; !slices.Equal(got, want) {
		t.Errorf("subtitles = %v, want = %v", got, want)
	}
	for _, s := range resp.GetSubtitles() {
		if !s.GetHashVerifiable() || s.GetDownloadCount() != 3 || !slices.Equal(s.GetMatches(), []string{"title"}) {
			t.Errorf("subtitle = %v", s)
		}
	}
	resp = h.search(t, pluginv1.MediaKind_MEDIA_KIND_MOVIE, "en", "fr")
	if got, want := providers(resp), []string{"a/1", "b/1", "fr-only/1", "movies/1"}; !slices.Equal(got, want) {
		t.Errorf("subtitles = %v, want = %v", got, want)
	}
	if got := h.fakes["fr-only"].langs; len(got) != 1 || len(got[0]) != 1 || got[0][0].Tag != "fr" {
		t.Errorf("fr-only searched in %v, want = [fr]", got)
	}
	req := pluginv1.SearchSubtitlesRequest_builder{
		Video: pluginv1.SubtitleVideo_builder{Title: proto.String("Up")}.Build(), Languages: []*pluginv1.SubtitleLanguage{language(provider.Language{Tag: "en"})},
		Providers: []string{"b"},
	}.Build()
	resp, err := h.p.SearchSubtitles(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := providers(resp), []string{"b/1"}; !slices.Equal(got, want) {
		t.Errorf("subtitles of b = %v, want = %v", got, want)
	}
}

func TestThrottling(t *testing.T) {
	h := newHarness(t)
	h.configure(t, allEnabled)
	h.fakes["a"].err = provider.Errorf(provider.DownloadLimitExceeded, "limit")
	h.fakes["b"].err = provider.Errorf(provider.TooManyRequests, "slow down")
	resp := h.search(t, pluginv1.MediaKind_MEDIA_KIND_MOVIE, "en")
	if n := len(resp.GetFailures()); n != 2 {
		t.Fatalf("failures = %d, want = 2", n)
	}
	for _, f := range resp.GetFailures() {
		switch f.GetProvider() {
		case "a":
			if f.GetKind() != pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_DOWNLOAD_LIMIT || !f.GetThrottledUntil().AsTime().Equal(h.now.Add(6*time.Hour)) {
				t.Errorf("failure of a = %v", f)
			}
		case "b":
			// Too many requests throttle on the fifth within two minutes.
			if f.GetKind() != pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_TOO_MANY_REQUESTS || f.HasThrottledUntil() {
				t.Errorf("failure of b = %v", f)
			}
		}
	}
	for range 4 {
		h.search(t, pluginv1.MediaKind_MEDIA_KIND_MOVIE, "en")
	}
	if n := len(h.fakes["a"].langs); n != 1 {
		t.Errorf("searches of throttled a = %d, want = 1", n)
	}
	list, err := h.p.ListSubtitleProviders(t.Context(), &pluginv1.ListSubtitleProvidersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range list.GetProviders() {
		throttled := info.GetName() == "a" || info.GetName() == "b"
		if info.HasThrottledUntil() != throttled {
			t.Errorf("%s throttled = %v, want = %v", info.GetName(), info.HasThrottledUntil(), throttled)
		}
		if info.GetName() == "b" && info.GetThrottleReason() != "TooManyRequests" {
			t.Errorf("reason of b = %q, want = TooManyRequests", info.GetThrottleReason())
		}
	}
	h.now = h.now.Add(2 * time.Hour)
	h.search(t, pluginv1.MediaKind_MEDIA_KIND_MOVIE, "en")
	if n := len(h.fakes["b"].langs); n != 6 {
		t.Errorf("searches of b after its throttle = %d, want = 6", n)
	}
	if n := len(h.fakes["a"].langs); n != 1 {
		t.Errorf("searches of a within its throttle = %d, want = 1", n)
	}
}

func TestConfigure(t *testing.T) {
	h := newHarness(t)
	h.configure(t, `{"providers":[{"name":"a","enabled":true,"settings":{}},{"name":"b","enabled":true,"settings":{"key":"k"}}]}`)
	h.fakes["b"].err = provider.Errorf(provider.AuthenticationError, "who")
	resp := h.search(t, pluginv1.MediaKind_MEDIA_KIND_MOVIE, "en")
	kinds := map[string]pluginv1.SubtitleFailureKind{}
	for _, f := range resp.GetFailures() {
		kinds[f.GetProvider()] = f.GetKind()
	}
	want := map[string]pluginv1.SubtitleFailureKind{
		"a": pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_CONFIGURATION,
		"b": pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_AUTHENTICATION,
	}
	if len(kinds) != 2 || kinds["a"] != want["a"] || kinds["b"] != want["b"] {
		t.Errorf("failures = %v, want = %v", kinds, want)
	}
	// Saving the same settings keeps the throttles; changing them forgets
	// authentication and configuration failures.
	h.configure(t, `{"providers":[{"name":"a","enabled":true,"settings":{}},{"name":"b","enabled":true,"settings":{"key":"k"}}]}`)
	if _, ok := h.p.throttle.Throttled("b"); !ok {
		t.Error("b throttled = false after the same settings, want = true")
	}
	h.fakes["b"].err = nil
	h.configure(t, `{"providers":[{"name":"a","enabled":true,"settings":{"key":"k"}},{"name":"b","enabled":true,"settings":{"key":"l"}}]}`)
	for _, name := range []string{"a", "b"} {
		if _, ok := h.p.throttle.Throttled(name); ok {
			t.Errorf("%s throttled = true after new settings, want = false", name)
		}
	}
	for _, cfg := range []string{`{"providers":[{"name":"z","enabled":true}]}`, `{"providers":[{"name":"a"},{"name":"a"}]}`, `[`} {
		if _, err := h.p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(cfg)}.Build()); err == nil {
			t.Errorf("Configure(%s): no error", cfg)
		}
	}
}

func TestDownload(t *testing.T) {
	h := newHarness(t)
	h.configure(t, allEnabled)
	video := pluginv1.SubtitleVideo_builder{Title: proto.String("Up")}.Build()
	resp, err := h.p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String("a"), Id: proto.String("7"), Video: video}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.GetData()) != "7 Up" || resp.GetFormat() != "srt" || resp.GetLanguage().GetLanguage() != "en" {
		t.Errorf("download = %v", resp)
	}
	h.fakes["a"].err = provider.Errorf(provider.BadSubtitle, "not a subtitle")
	resp, err = h.p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String("a"), Id: proto.String("7")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	// A bad subtitle is blocklisted, not throttled for.
	if resp.GetFailure().GetKind() != pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_BAD_SUBTITLE || resp.GetFailure().HasThrottledUntil() {
		t.Errorf("failure = %v", resp.GetFailure())
	}
	if _, ok := h.p.throttle.Throttled("a"); ok {
		t.Error("a throttled after a bad subtitle")
	}
	h.fakes["a"].err = provider.Errorf(provider.DownloadLimitExceeded, "limit")
	for range 2 {
		resp, err = h.p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String("a"), Id: proto.String("7")}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if f := resp.GetFailure(); f.GetKind() != pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_DOWNLOAD_LIMIT || !f.HasThrottledUntil() {
			t.Errorf("failure = %v", f)
		}
	}
	if _, err := h.p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String("z"), Id: proto.String("7")}.Build()); !errors.Is(err, errNotEnabled) {
		t.Errorf("download from an unknown provider: err = %v, want = %v", err, errNotEnabled)
	}
}

func TestListSettings(t *testing.T) {
	h := newHarness(t)
	h.configure(t, `{"providers":[{"name":"a","enabled":true,"settings":{"key":"k"}}]}`)
	list, err := h.p.ListSubtitleProviders(t.Context(), &pluginv1.ListSubtitleProvidersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	a := list.GetProviders()[0]
	if a.GetName() != "a" || !a.GetEnabled() || list.GetProviders()[1].GetEnabled() || !a.GetHashVerifiable() {
		t.Errorf("providers = %v", list.GetProviders())
	}
	key, n := a.GetSettings()[0], a.GetSettings()[1]
	if !key.GetSecret() || key.GetType() != pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT ||
		n.GetType() != pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER || n.GetDefaultJson() != "3" {
		t.Errorf("settings = %v", a.GetSettings())
	}
}

func TestDefinitions(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range definitions() {
		if d.Name == "" || seen[d.Name] || d.New == nil || d.DisplayName == "" {
			t.Errorf("definition %+v", d)
		}
		seen[d.Name] = true
	}
}

func TestConfigureNetwork(t *testing.T) {
	h := newHarness(t)
	var envs []provider.Env
	def := &h.p.defs[0]
	def.New = func(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
		envs = append(envs, env)
		return h.fakes["a"], nil
	}
	h.configure(t, `{"providers":[{"name":"a","enabled":true}],
		"proxy":{"kind":"socks5","host":"proxy:1080"},
		"flaresolverr":{"url":"http://flaresolverr:8191"},
		"captcha":{"service":"anti-captcha","key":"k"}}`)
	if len(envs) != 1 || envs[0].HTTP == nil || envs[0].Captcha == nil {
		t.Fatalf("got env = %+v, want a client and a captcha solver", envs)
	}
	h.configure(t, `{"providers":[{"name":"a","enabled":true}]}`)
	if envs[1].Captcha != nil {
		t.Errorf("got captcha solver = %v, want none", envs[1].Captcha)
	}
	for _, cfg := range []string{
		`{"proxy":{"kind":"ftp","host":"proxy:21"}}`,
		`{"proxy":{"kind":"http","host":"proxy"}}`,
		`{"flaresolverr":{"url":"flaresolverr:8191"}}`,
		`{"captcha":{"service":"2captcha","key":"k"}}`,
	} {
		if _, err := h.p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(cfg)}.Build()); err == nil {
			t.Errorf("Configure(%s) got = nil, want an error", cfg)
		}
	}
}
