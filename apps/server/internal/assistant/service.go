package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/assistant/intent"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1/assistantv1connect"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
	"github.com/mavioai/mavio/libs/subtitle"
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
	// Subtitles reads a text subtitle stream of an item by its index,
	// once Items has shown the user may see the item; nil reads none.
	Subtitles func(ctx context.Context, itemID core.ID, index int) (*subtitle.Subtitle, error)
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
	// VoiceSessions returns the ready plugin running voice sessions; nil
	// has none.
	VoiceSessions func() (VoicePlugin, bool)
	// ServerName names the server to the model.
	ServerName func() string
	// SpeechDir holds the speech of answers kept with their conversations;
	// empty keeps none.
	SpeechDir string
	// FFmpeg encodes the speech kept, and converts it for clients that do
	// not play Opus; empty keeps it as WAV and converts none.
	FFmpeg string
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
	// runs are the runs a voice session can steer and abort.
	runs  map[runKey]*run
	voice voices
	rec   *recorder
	// keeping are the answers' speech being kept.
	keeping sync.WaitGroup
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
		dialogs: map[core.ID]*dialog{}, calls: map[string]callResult{}, turns: map[string]*aiTurn{}, runs: map[runKey]*run{},
		voice: voices{byURL: map[string]*voiceSession{}, byToken: map[string]*voiceSession{}}, rec: newRecorder(cfg.SpeechDir, cfg.FFmpeg, cfg.Logger),
	}
	s.tool = map[string]*tool{}
	for _, t := range tools() {
		s.tool[t.name] = t
	}
	return s
}

// Run watches the model's tool calls and deletes conversations past their
// retention, until ctx ends, when it ends the voice sessions.
func (s *Service) Run(ctx context.Context) {
	defer s.endVoices()
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

// Close waits for the answers' speech being kept and closes the folder
// it is kept in; the Service serves nothing after.
func (s *Service) Close() error {
	s.keeping.Wait()
	return s.rec.close()
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
	if s.cfg.VoiceSessions != nil {
		_, voice := s.cfg.VoiceSessions()
		resp.SetVoice(voice)
	}
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
