package assistant

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var accesses = map[core.AssistantAccess]assistantv1.AssistantAccess{
	core.AssistantAccessAll:            assistantv1.AssistantAccess_ASSISTANT_ACCESS_ALL,
	core.AssistantAccessAdministrators: assistantv1.AssistantAccess_ASSISTANT_ACCESS_ADMINISTRATORS,
	core.AssistantAccessSelected:       assistantv1.AssistantAccess_ASSISTANT_ACCESS_SELECTED,
}

func settingsToProto(s core.AssistantSettings) *assistantv1.AssistantSettings {
	users := make([]string, 0, len(s.AIUsers))
	for _, id := range s.AIUsers {
		users = append(users, id.String())
	}
	return assistantv1.AssistantSettings_builder{
		Enabled: &s.Enabled, Model: &s.Model, CodeMode: &s.CodeMode, AiAccess: new(accesses[s.AIAccess]), AiUserIds: users,
		IdleMinutes: new(int32(s.IdleTime / time.Minute)), RetentionDays: new(int32(s.RetentionDays)), Memory: &s.Memory,
		Hints: &s.Hints, Speech: speechToProto(s.Speech), LongestPause: durationpb.New(s.LongestPause),
		CommitDelay: durationpb.New(s.CommitDelay), SessionIdle: durationpb.New(s.SessionIdle),
	}.Build()
}

func speechToProto(list []core.AssistantSpeech) []*assistantv1.AssistantSpeech {
	out := make([]*assistantv1.AssistantSpeech, 0, len(list))
	for _, sp := range list {
		out = append(out, assistantv1.AssistantSpeech_builder{Language: &sp.Language, Recognizer: &sp.Recognizer, Voice: &sp.Voice}.Build())
	}
	return out
}

func settingsFromProto(p *assistantv1.AssistantSettings) (core.AssistantSettings, error) {
	s := core.AssistantSettings{
		Enabled: p.GetEnabled(), Model: p.GetModel(), CodeMode: p.GetCodeMode(),
		IdleTime: time.Duration(p.GetIdleMinutes()) * time.Minute, RetentionDays: int(p.GetRetentionDays()),
		Memory: p.GetMemory(), Hints: p.GetHints(), LongestPause: p.GetLongestPause().AsDuration(),
		CommitDelay: p.GetCommitDelay().AsDuration(), SessionIdle: p.GetSessionIdle().AsDuration(),
	}
	for k, v := range accesses {
		if v == p.GetAiAccess() {
			s.AIAccess = k
		}
	}
	for _, u := range p.GetAiUserIds() {
		id, err := core.ParseID(u)
		if err != nil {
			return s, connect.NewError(connect.CodeInvalidArgument, err)
		}
		s.AIUsers = append(s.AIUsers, id)
	}
	for _, sp := range p.GetSpeech() {
		s.Speech = append(s.Speech, core.AssistantSpeech{Language: sp.GetLanguage(), Recognizer: sp.GetRecognizer(), Voice: sp.GetVoice()})
	}
	return s, nil
}

func admin(ctx context.Context) (auth.Principal, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return p, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	if !p.User.IsAdmin() {
		return p, connect.NewError(connect.CodePermissionDenied, errors.New("administrator required"))
	}
	return p, nil
}

// GetAssistantSettings returns the server's assistant settings.
func (s *Service) GetAssistantSettings(ctx context.Context, _ *assistantv1.GetAssistantSettingsRequest) (*assistantv1.GetAssistantSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	resp := assistantv1.GetAssistantSettingsResponse_builder{Settings: settingsToProto(s.cfg.Settings())}.Build()
	if s.cfg.VoiceSessions == nil {
		return resp, nil
	}
	vp, ok := s.cfg.VoiceSessions()
	if !ok {
		return resp, nil
	}
	list, err := vp.Client.ListVoiceServices(ctx, &pluginv1.ListVoiceServicesRequest{})
	if err != nil {
		s.log.WarnContext(ctx, "assistant: list voice services", "plugin", vp.ID, "err", err)
		return resp, nil
	}
	resp.SetRecognizers(voiceEngines(vp.ID, list.GetRecognizers()))
	resp.SetVoices(voiceEngines(vp.ID, list.GetVoices()))
	return resp, nil
}

// voiceEngines returns the enabled recognizers or voices of a plugin.
func voiceEngines(plugin string, list []*pluginv1.VoiceEngineInfo) []*assistantv1.VoiceEngine {
	var out []*assistantv1.VoiceEngine
	for _, e := range list {
		if e.GetEnabled() {
			out = append(out, assistantv1.VoiceEngine_builder{
				Id: new(plugin + "/" + e.GetId()), Name: new(cmp.Or(e.GetName(), e.GetId())), Languages: e.GetLanguages(), Ready: new(e.GetReady()),
			}.Build())
		}
	}
	return out
}

// UpdateAssistantSettings replaces the server's assistant settings.
func (s *Service) UpdateAssistantSettings(ctx context.Context, req *assistantv1.UpdateAssistantSettingsRequest) (*assistantv1.UpdateAssistantSettingsResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if s.cfg.Change == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the settings cannot be changed"))
	}
	set, err := settingsFromProto(req.GetSettings())
	if err != nil {
		return nil, err
	}
	if err := set.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	set, err = s.cfg.Change(ctx, set)
	if err != nil {
		return nil, err
	}
	if s.cfg.Activity != nil {
		s.cfg.Activity.Record(ctx, core.Activity{Type: "settings.updated", UserID: p.User.ID, Title: p.User.Name + " changed the assistant settings"})
	}
	return assistantv1.UpdateAssistantSettingsResponse_builder{Settings: settingsToProto(set)}.Build(), nil
}

// hints lists titles being watched, recently added titles and their
// people, and the user's devices.
func (s *Service) hints(ctx context.Context, p auth.Principal) []string {
	c := &call{s: s, ctx: ctx, user: p.User}
	var out []string
	add := func(names ...string) {
		for _, n := range names {
			if n != "" && !slices.Contains(out, n) && len(out) < 100 {
				out = append(out, n)
			}
		}
	}
	addItem := func(it *libraryv1.Item) {
		if it.GetSeriesName() != "" {
			add(it.GetSeriesName())
		} else {
			add(it.GetName())
		}
	}
	if r, err := c.listResume(listArgs{Limit: 10}); err == nil {
		for _, it := range r.(itemsResult).items {
			addItem(it)
		}
	}
	if r, err := c.listNextUp(listArgs{Limit: 10}); err == nil {
		for _, it := range r.(itemsResult).items {
			addItem(it)
		}
	}
	if r, err := s.cfg.Services.Items.ListLatestItems(ctx, libraryv1.ListLatestItemsRequest_builder{Limit: new(int32(20))}.Build()); err == nil {
		for _, it := range r.GetItems() {
			addItem(it)
		}
	}
	if list, err := c.sessions(); err == nil {
		for _, sess := range list {
			add(sess.GetDeviceName())
		}
	}
	if r, err := s.cfg.Services.Libraries.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{}); err == nil {
		for _, l := range r.GetLibraries() {
			add(l.GetName())
		}
	}
	return out
}
