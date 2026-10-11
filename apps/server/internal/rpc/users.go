package rpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

var subtitleModes = map[core.SubtitleMode]userv1.SubtitleMode{
	core.SubtitlesDefault:    userv1.SubtitleMode_SUBTITLE_MODE_UNSPECIFIED,
	core.SubtitlesAlways:     userv1.SubtitleMode_SUBTITLE_MODE_ALWAYS,
	core.SubtitlesForeign:    userv1.SubtitleMode_SUBTITLE_MODE_FOREIGN,
	core.SubtitlesForcedOnly: userv1.SubtitleMode_SUBTITLE_MODE_FORCED,
	core.SubtitlesNone:       userv1.SubtitleMode_SUBTITLE_MODE_NONE,
	core.SubtitlesSmart:      userv1.SubtitleMode_SUBTITLE_MODE_SMART,
}

// userToProto converts a user, leaving out its credentials.
func userToProto(u *core.User) *userv1.User {
	libraries := make([]string, len(u.Policy.Libraries))
	for i, id := range u.Policy.Libraries {
		libraries[i] = id.String()
	}
	var lastLogin *timestamppb.Timestamp
	if u.LastLoginAt != nil {
		lastLogin = timestamppb.New(*u.LastLoginAt)
	}
	return userv1.User_builder{
		Id:           new(u.ID.String()),
		Name:         &u.Name,
		AuthProvider: &u.AuthProvider,
		Admin:        &u.Admin,
		Disabled:     &u.Disabled,
		Policy: userv1.UserPolicy_builder{
			AllLibraries:        new(u.Policy.Libraries == nil),
			LibraryIds:          libraries,
			MaxParentalRating:   int32Ptr(u.Policy.MaxParentalRating),
			BlockUnrated:        &u.Policy.BlockUnrated,
			AllowTranscoding:    &u.Policy.AllowTranscoding,
			AllowDownload:       &u.Policy.AllowDownload,
			MaxStreamingBitrate: &u.Policy.MaxStreamingBitrate,
			MaxSessions:         new(int32(u.Policy.MaxSessions)),
			RequestMovies:       &u.Policy.Requests.Movies,
			RequestSeries:       &u.Policy.Requests.Series,
			AutoApproveMovies:   &u.Policy.Requests.AutoApproveMovies,
			AutoApproveSeries:   &u.Policy.Requests.AutoApproveSeries,
			ReportIssues:        &u.Policy.Requests.ReportIssues,
			MovieQuota:          quotaToProto(u.Policy.MovieQuota),
			SeriesQuota:         quotaToProto(u.Policy.SeriesQuota),
		}.Build(),
		Preferences: userv1.UserPreferences_builder{
			AudioLanguages:        u.Preferences.AudioLanguages,
			SubtitleLanguages:     u.Preferences.SubtitleLanguages,
			SubtitleMode:          new(subtitleModes[u.Preferences.SubtitleMode]),
			PlayDefaultAudioTrack: &u.Preferences.PlayDefaultAudioTrack,
		}.Build(),
		CreateTime:    timestamppb.New(u.CreatedAt),
		LastLoginTime: lastLogin,
	}.Build()
}

// quotaToProto converts a user's quota; nil for the server's.
func quotaToProto(q *core.Quota) *userv1.RequestQuota {
	if q == nil {
		return nil
	}
	return userv1.RequestQuota_builder{Limit: new(int32(q.Limit)), Days: new(int32(q.Days))}.Build()
}
