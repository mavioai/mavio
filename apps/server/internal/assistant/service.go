package assistant

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/assistant/intent"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/speech"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1/assistantv1connect"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// Services are the server's own services the tools call, as the user of
// the request: the assistant reaches nothing the UI does not.
type Services struct {
	Items     libraryv1connect.ItemServiceHandler
	Libraries libraryv1connect.LibraryServiceHandler
	Playlists libraryv1connect.PlaylistServiceHandler
	Sessions  sessionv1connect.SessionServiceHandler
	UserData  userv1connect.UserDataServiceHandler
	// Discover and Requests request titles the server lacks; nil requests
	// none.
	Discover acquisitionv1connect.DiscoverServiceHandler
	Requests acquisitionv1connect.RequestServiceHandler
}

// Config configures a Service.
type Config struct {
	Services Services
	// Users finds the users of the model's tool calls.
	Users core.UserRepository
	// Settings returns the server's assistant settings, and Change changes
	// them; nil Change refuses changes.
	Settings func() core.AssistantSettings
	Change   func(ctx context.Context, s core.AssistantSettings) (core.AssistantSettings, error)
	// Grammar is the fast path's grammar; nil understands nothing without
	// a model.
	Grammar *intent.Grammar
	// Agents returns the plugin keeping conversations with models; nil
	// has none.
	Agents func() (pluginv1connect.AgentSessionServiceClient, bool)
	// Recognizers and Voices return the started speech plugins; nil have
	// none.
	Recognizers func() []speech.RecognizerPlugin
	Voices      func() []speech.VoicePlugin
	// ServerName names the server to the model.
	ServerName func() string
	// Key signs the listening URLs.
	Key []byte
	// Activity records what the assistant does; nil records nothing.
	Activity *activity.Log
	Logger   *slog.Logger
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Service implements assistantv1connect.AssistantServiceHandler.
type Service struct {
	cfg  Config
	log  *slog.Logger
	now  func() time.Time
	tool map[string]*tool

	mu      sync.Mutex
	dialogs map[core.ID]*dialog
	// calls are the results of the model's tool calls by call ID, kept a
	// day so that a call handed again is answered without running again.
	calls map[string]callResult
	// turns are the AI path's running turns by conversation.
	turns map[string]*aiTurn
	// listens are the listening tokens used, until they expire.
	listens map[string]time.Time
	vad     vadLoader
}

var _ assistantv1connect.AssistantServiceHandler = (*Service)(nil)

// New returns a Service; Run must run for the AI path's tool calls and
// for retention.
func New(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Settings == nil {
		cfg.Settings = core.DefaultAssistantSettings
	}
	if cfg.ServerName == nil {
		cfg.ServerName = func() string { return "Mavio" }
	}
	s := &Service{
		cfg: cfg, log: cfg.Logger, now: cfg.Now,
		dialogs: map[core.ID]*dialog{}, calls: map[string]callResult{}, turns: map[string]*aiTurn{},
		listens: map[string]time.Time{},
	}
	s.tool = map[string]*tool{}
	for _, t := range tools() {
		s.tool[t.name] = t
	}
	return s
}

// Run watches the model's tool calls and deletes conversations past their
// retention, until ctx ends.
func (s *Service) Run(ctx context.Context) {
	go s.watchCalls(ctx)
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		s.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// caller returns the request's principal, refusing a disabled assistant.
func (s *Service) caller(ctx context.Context) (auth.Principal, core.AssistantSettings, error) {
	p, ok := auth.FromContext(ctx)
	if !ok || p.User.ID.IsZero() {
		return p, core.AssistantSettings{}, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	set := s.cfg.Settings()
	if !set.Enabled {
		return p, set, connect.NewError(connect.CodeFailedPrecondition, errors.New("the assistant is off"))
	}
	return p, set, nil
}

// agents returns the agent plugin when the user may use the AI path.
func (s *Service) agents(set core.AssistantSettings, u *core.User) (pluginv1connect.AgentSessionServiceClient, bool) {
	if !set.AllowsAI(u) || s.cfg.Agents == nil {
		return nil, false
	}
	return s.cfg.Agents()
}

// GetAssistant tells what the assistant can do for the user.
func (s *Service) GetAssistant(ctx context.Context, req *assistantv1.GetAssistantRequest) (*assistantv1.GetAssistantResponse, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	set := s.cfg.Settings()
	resp := &assistantv1.GetAssistantResponse{}
	resp.SetEnabled(set.Enabled)
	if !set.Enabled {
		return resp, nil
	}
	_, ai := s.agents(set, &p.User)
	resp.SetAi(ai)
	resp.SetMemory(ai && set.Memory)
	var recognizers []*assistantv1.Recognizer
	for _, r := range s.recognizers(ctx) {
		recognizers = append(recognizers, assistantv1.Recognizer_builder{
			Id: new(r.ID), Name: new(r.Info.GetDisplayName()), Languages: r.Info.GetLanguages(),
		}.Build())
	}
	resp.SetRecognizers(recognizers)
	var voices []*assistantv1.Voice
	for _, v := range s.voices(ctx) {
		voices = append(voices, assistantv1.Voice_builder{
			Id: new(v.ID), Name: new(v.Info.GetName()), Languages: v.Info.GetLanguages(),
		}.Build())
	}
	resp.SetVoices(voices)
	resp.SetDefaultVoices(voiceChoices(set.Voices))
	if g := s.cfg.Grammar; g != nil {
		if ex, err := g.Examples(ctx, intentv1.ExamplesRequest_builder{Language: new(req.GetLanguage())}.Build()); err == nil {
			resp.SetExamples(ex.GetExamples())
		}
		if l, err := g.Languages(ctx); err == nil {
			resp.SetLanguages(l.GetLanguages())
		}
	}
	return resp, nil
}

func (s *Service) recognizers(ctx context.Context) []speech.Recognizer {
	if s.cfg.Recognizers == nil {
		return nil
	}
	return speech.Recognizers(ctx, s.cfg.Recognizers(), s.log)
}

func (s *Service) voices(ctx context.Context) []speech.Voice {
	if s.cfg.Voices == nil {
		return nil
	}
	return speech.Voices(ctx, s.cfg.Voices(), s.log)
}

func voiceChoices(m map[string]string) []*assistantv1.AssistantVoiceChoice {
	langs := make([]string, 0, len(m))
	for l := range m {
		langs = append(langs, l)
	}
	slices.Sort(langs)
	out := make([]*assistantv1.AssistantVoiceChoice, 0, len(langs))
	for _, l := range langs {
		out = append(out, assistantv1.AssistantVoiceChoice_builder{Language: new(l), Voice: new(m[l])}.Build())
	}
	return out
}

// answerLanguage picks the language of answers: the grammar's when it matched,
// else the context's first, else English.
func answerLanguage(intentLanguage string, ctx *assistantv1.AskContext) string {
	if intentLanguage != "" {
		return intentLanguage
	}
	for _, l := range ctx.GetLanguages() {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "en"
}

// record adds the assistant's action to the activity log, as the user's.
func (s *Service) record(ctx context.Context, u *core.User, kind, title string, item core.ID, attrs map[string]string) {
	if s.cfg.Activity == nil {
		return
	}
	if attrs == nil {
		attrs = map[string]string{}
	}
	attrs["via"] = "assistant"
	s.cfg.Activity.Record(ctx, core.Activity{Type: kind, UserID: u.ID, ItemID: item, Title: title, Attributes: attrs})
}

// Bind sets the services the tools call, which the HTTP handler builds
// after the Service; it is called before the Service serves.
func (s *Service) Bind(svc Services) { s.cfg.Services = svc }
