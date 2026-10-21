package app

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
)

// fakeSubtitles serves a fake server's subtitle providers, translators,
// profiles, their assignments and the subtitle settings.
type fakeSubtitles struct {
	acquisitionv1connect.UnimplementedSubtitleServiceHandler

	mu          sync.Mutex
	profiles    []*acquisitionv1.SubtitleProfile
	assignments map[string]string
	settings    *acquisitionv1.SubtitleSettings
	next        int
	// items are the items' subtitles; calls the changes asked of them.
	items map[string][]*acquisitionv1.ItemSubtitle
	calls []string
}

func newFakeSubtitles() *fakeSubtitles {
	return &fakeSubtitles{
		profiles: []*acquisitionv1.SubtitleProfile{acquisitionv1.SubtitleProfile_builder{
			Id: new("sp1"), Name: new("French"), Items: []*acquisitionv1.SubtitleProfileItem{
				acquisitionv1.SubtitleProfileItem_builder{Id: new(int32(1)), Language: new("fr"), Forced: new(true)}.Build(),
				acquisitionv1.SubtitleProfileItem_builder{Id: new(int32(2)), Language: new("en")}.Build(),
			},
		}.Build()},
		assignments: map[string]string{"movies": "sp1"},
		settings: acquisitionv1.SubtitleSettings_builder{
			MinimumScore: new(int32(80)), MinimumScoreMovie: new(int32(70)), UseEmbedded: new(true), Mods: []string{"common"},
			Translation: acquisitionv1.SubtitleTranslationSettings_builder{Translator: new("google"), MaxRetries: new(int32(3))}.Build(),
		}.Build(),
	}
}

func (*fakeSubtitles) ListSubtitleProviders(context.Context, *acquisitionv1.ListSubtitleProvidersRequest) (*acquisitionv1.ListSubtitleProvidersResponse, error) {
	return acquisitionv1.ListSubtitleProvidersResponse_builder{
		Providers: []*acquisitionv1.SubtitleProviderState{
			acquisitionv1.SubtitleProviderState_builder{
				Id: new("os"), PluginId: new("dev.mavio.tmdb"), Name: new("opensubtitles"), DisplayName: new("OpenSubtitles"), Enabled: new(true),
				Movies: new(true), Episodes: new(true), ThrottledUntil: timestamppb.New(time.Now().Add(2 * time.Hour)), ThrottleReason: new("too many requests"),
			}.Build(),
			acquisitionv1.SubtitleProviderState_builder{Id: new("pod"), PluginId: new("dev.mavio.tmdb"), Name: new("podnapisi"), Movies: new(true)}.Build(),
		},
		FailedPlugins: []string{"dev.mavio.subs"},
	}.Build(), nil
}

func (*fakeSubtitles) ListSubtitleTranslators(context.Context, *acquisitionv1.ListSubtitleTranslatorsRequest) (*acquisitionv1.ListSubtitleTranslatorsResponse, error) {
	return acquisitionv1.ListSubtitleTranslatorsResponse_builder{Translators: []*acquisitionv1.SubtitleTranslatorState{
		acquisitionv1.SubtitleTranslatorState_builder{Id: new("google"), PluginId: new("dev.mavio.tmdb"), Name: new("Google Translate"), Enabled: new(true)}.Build(),
	}}.Build(), nil
}

func (s *fakeSubtitles) ListSubtitleProfiles(context.Context, *acquisitionv1.ListSubtitleProfilesRequest) (*acquisitionv1.ListSubtitleProfilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListSubtitleProfilesResponse_builder{Profiles: slices.Clone(s.profiles)}.Build(), nil
}

func (s *fakeSubtitles) CreateSubtitleProfile(_ context.Context, r *acquisitionv1.CreateSubtitleProfileRequest) (*acquisitionv1.CreateSubtitleProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	p := proto.CloneOf(r.GetProfile())
	p.SetId(fmt.Sprintf("new%d", s.next))
	s.profiles = append(s.profiles, p)
	return acquisitionv1.CreateSubtitleProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeSubtitles) UpdateSubtitleProfile(_ context.Context, r *acquisitionv1.UpdateSubtitleProfileRequest) (*acquisitionv1.UpdateSubtitleProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := proto.CloneOf(r.GetProfile())
	for i, x := range s.profiles {
		if x.GetId() == p.GetId() {
			s.profiles[i] = p
		}
	}
	return acquisitionv1.UpdateSubtitleProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeSubtitles) DeleteSubtitleProfile(_ context.Context, r *acquisitionv1.DeleteSubtitleProfileRequest) (*acquisitionv1.DeleteSubtitleProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles = slices.DeleteFunc(s.profiles, func(p *acquisitionv1.SubtitleProfile) bool { return p.GetId() == r.GetId() })
	return &acquisitionv1.DeleteSubtitleProfileResponse{}, nil
}

func (s *fakeSubtitles) ListSubtitleProfileAssignments(context.Context, *acquisitionv1.ListSubtitleProfileAssignmentsRequest) (*acquisitionv1.ListSubtitleProfileAssignmentsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []*acquisitionv1.SubtitleProfileAssignment
	for target, profile := range s.assignments {
		list = append(list, acquisitionv1.SubtitleProfileAssignment_builder{TargetId: &target, ProfileId: &profile}.Build())
	}
	return acquisitionv1.ListSubtitleProfileAssignmentsResponse_builder{Assignments: list}.Build(), nil
}

func (s *fakeSubtitles) AssignSubtitleProfile(_ context.Context, r *acquisitionv1.AssignSubtitleProfileRequest) (*acquisitionv1.AssignSubtitleProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.GetProfileId() == "" {
		delete(s.assignments, r.GetTargetId())
	} else {
		s.assignments[r.GetTargetId()] = r.GetProfileId()
	}
	return &acquisitionv1.AssignSubtitleProfileResponse{}, nil
}

func (s *fakeSubtitles) GetSubtitleSettings(context.Context, *acquisitionv1.GetSubtitleSettingsRequest) (*acquisitionv1.GetSubtitleSettingsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.GetSubtitleSettingsResponse_builder{Settings: proto.CloneOf(s.settings)}.Build(), nil
}

func (s *fakeSubtitles) UpdateSubtitleSettings(_ context.Context, r *acquisitionv1.UpdateSubtitleSettingsRequest) (*acquisitionv1.UpdateSubtitleSettingsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = proto.CloneOf(r.GetSettings())
	return acquisitionv1.UpdateSubtitleSettingsResponse_builder{Settings: proto.CloneOf(s.settings)}.Build(), nil
}
