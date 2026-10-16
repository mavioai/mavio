package assistant

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
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
		Recognizer: &s.Recognizer, Voices: voiceChoices(s.Voices), Hints: &s.Hints,
	}.Build()
}

func settingsFromProto(p *assistantv1.AssistantSettings) (core.AssistantSettings, error) {
	s := core.AssistantSettings{
		Enabled: p.GetEnabled(), Model: p.GetModel(), CodeMode: p.GetCodeMode(),
		IdleTime: time.Duration(p.GetIdleMinutes()) * time.Minute, RetentionDays: int(p.GetRetentionDays()),
		Memory: p.GetMemory(), Recognizer: p.GetRecognizer(), Hints: p.GetHints(),
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
	if vs := p.GetVoices(); len(vs) > 0 {
		s.Voices = map[string]string{}
		for _, v := range vs {
			s.Voices[v.GetLanguage()] = v.GetVoice()
		}
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
	return assistantv1.GetAssistantSettingsResponse_builder{Settings: settingsToProto(s.cfg.Settings())}.Build(), nil
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

// GetAssistantHints returns phrases recognizers are likely to hear.
func (s *Service) GetAssistantHints(ctx context.Context, _ *assistantv1.GetAssistantHintsRequest) (*assistantv1.GetAssistantHintsResponse, error) {
	p, set, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	resp := &assistantv1.GetAssistantHintsResponse{}
	if !set.Hints {
		return resp, nil
	}
	resp.SetPhrases(s.hints(ctx, p))
	return resp, nil
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
