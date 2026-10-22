package app

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakePlugins serves the server's plugins: one that resets passwords and
// TMDB, whose catalog offers a newer version, and those its catalog
// offers.
type fakePlugins struct {
	systemv1connect.UnimplementedPluginManagementServiceHandler
	f *fakeCore
}

// fakeAuth serves the server's API keys.
type fakeAuth struct {
	authv1connect.UnimplementedAuthServiceHandler
	f *fakeCore
}

// fakeGeneralSettings are the general settings a fake server starts with.
func fakeGeneralSettings() *systemv1.GeneralSettings {
	return systemv1.GeneralSettings_builder{
		Network:         systemv1.NetworkSettings_builder{ServerName: new("Living Room"), LocalDiscovery: new(true)}.Build(),
		SessionIdleDays: new(int32(90)),
	}.Build()
}

// fakeTranscodingSettings are the transcoding settings a fake server
// starts with.
func fakeTranscodingSettings() *playbackv1.TranscodingSettings {
	return playbackv1.TranscodingSettings_builder{
		HardwareAcceleration: new(playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO), HardwareEncoding: new(true),
		H264Crf: new(int32(23)), H265Crf: new(int32(28)), TonemapAlgorithm: new("bt2390"), TonemapRange: new("auto"),
		TonemapDesat: new(0.0), DeinterlaceMethod: new("yadif"), DownmixBoost: new(2.0),
	}.Build()
}

// fakePlayback serves the server's transcoding settings.
type fakePlayback struct {
	playbackv1connect.UnimplementedPlaybackServiceHandler
	f *fakeCore
}

// fakeApiKeyList are the API keys a fake server starts with.
func fakeApiKeyList() []*authv1.ApiKey {
	return []*authv1.ApiKey{
		authv1.ApiKey_builder{Id: new("k1"), Name: new("Home Assistant"), UserId: new("u1"), CreateTime: tsAgo(30 * 24 * time.Hour), LastUseTime: tsAgo(2 * time.Hour)}.Build(),
	}
}

func (s *fakeSystem) GetGeneralSettings(context.Context, *systemv1.GetGeneralSettingsRequest) (*systemv1.GetGeneralSettingsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return systemv1.GetGeneralSettingsResponse_builder{Settings: proto.CloneOf(s.f.general)}.Build(), nil
}

func (s *fakeSystem) UpdateGeneralSettings(_ context.Context, r *systemv1.UpdateGeneralSettingsRequest) (*systemv1.UpdateGeneralSettingsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.general = proto.CloneOf(r.GetSettings())
	return systemv1.UpdateGeneralSettingsResponse_builder{Settings: proto.CloneOf(s.f.general)}.Build(), nil
}

func (s *fakePlayback) GetTranscodingSettings(context.Context, *playbackv1.GetTranscodingSettingsRequest) (*playbackv1.GetTranscodingSettingsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return playbackv1.GetTranscodingSettingsResponse_builder{
		Settings: proto.CloneOf(s.f.transcoding),
		AvailableHardwareAccelerations: []playbackv1.HardwareAcceleration{
			playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO, playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE,
			playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_VIDEOTOOLBOX,
		},
	}.Build(), nil
}

func (s *fakePlayback) UpdateTranscodingSettings(_ context.Context, r *playbackv1.UpdateTranscodingSettingsRequest) (*playbackv1.UpdateTranscodingSettingsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.transcoding = proto.CloneOf(r.GetSettings())
	return playbackv1.UpdateTranscodingSettingsResponse_builder{Settings: proto.CloneOf(s.f.transcoding)}.Build(), nil
}

func (s *fakeAuth) ListApiKeys(context.Context, *authv1.ListApiKeysRequest) (*authv1.ListApiKeysResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return authv1.ListApiKeysResponse_builder{ApiKeys: slices.Clone(s.f.apiKeys)}.Build(), nil
}

func (s *fakeAuth) CreateApiKey(_ context.Context, r *authv1.CreateApiKeyRequest) (*authv1.CreateApiKeyResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	key := authv1.ApiKey_builder{Id: new("k-new"), Name: new(r.GetName()), UserId: new("u1"), CreateTime: tsAgo(0)}.Build()
	s.f.apiKeys = append(s.f.apiKeys, key)
	return authv1.CreateApiKeyResponse_builder{ApiKey: key, Token: new("mk_secret-token")}.Build(), nil
}

func (s *fakeAuth) RevokeApiKey(_ context.Context, r *authv1.RevokeApiKeyRequest) (*authv1.RevokeApiKeyResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.apiKeys = slices.DeleteFunc(s.f.apiKeys, func(k *authv1.ApiKey) bool { return k.GetId() == r.GetId() })
	return &authv1.RevokeApiKeyResponse{}, nil
}
