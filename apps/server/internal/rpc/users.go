package rpc

import (
	"context"
	"slices"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
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

var restrictions = map[core.RestrictionProfile]userv1.RestrictionProfile{
	core.RestrictionNone:         userv1.RestrictionProfile_RESTRICTION_PROFILE_UNSPECIFIED,
	core.RestrictionYoungerChild: userv1.RestrictionProfile_RESTRICTION_PROFILE_YOUNGER_CHILD,
	core.RestrictionOlderChild:   userv1.RestrictionProfile_RESTRICTION_PROFILE_OLDER_CHILD,
	core.RestrictionTeen:         userv1.RestrictionProfile_RESTRICTION_PROFILE_TEEN,
}

func restrictionFromProto(r userv1.RestrictionProfile) core.RestrictionProfile {
	for k, v := range restrictions {
		if v == r {
			return k
		}
	}
	return core.RestrictionNone
}

// describeUser converts a user, telling which credentials it has.
func describeUser(ctx context.Context, store core.Store, u *core.User) (*userv1.User, error) {
	creds, err := store.Credentials().List(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return userToProto(u, creds), nil
}

// userToProto converts a user with its credentials, leaving out their
// secrets.
func userToProto(u *core.User, creds []core.Credential) *userv1.User {
	var passkeys []*userv1.Passkey
	for i := range creds {
		if creds[i].Kind == core.CredentialPasskey {
			passkeys = append(passkeys, passkeyToProto(&creds[i]))
		}
	}
	policy := accessToProto(&u.Access)
	policy.SetAllowTranscoding(u.Policy.AllowTranscoding)
	policy.SetMaxStreamingBitrate(u.Policy.MaxStreamingBitrate)
	policy.SetMaxSessions(int32(u.Policy.MaxSessions))
	policy.SetRequestMovies(u.Policy.Requests.Movies)
	policy.SetRequestSeries(u.Policy.Requests.Series)
	policy.SetAutoApproveMovies(u.Policy.Requests.AutoApproveMovies)
	policy.SetAutoApproveSeries(u.Policy.Requests.AutoApproveSeries)
	policy.SetReportIssues(u.Policy.Requests.ReportIssues)
	policy.SetMovieQuota(quotaToProto(u.Policy.MovieQuota))
	policy.SetSeriesQuota(quotaToProto(u.Policy.SeriesQuota))
	var lastLogin *timestamppb.Timestamp
	if u.LastLoginAt != nil {
		lastLogin = timestamppb.New(*u.LastLoginAt)
	}
	return userv1.User_builder{
		Id:           new(u.ID.String()),
		Name:         &u.Name,
		AuthProvider: &u.AuthProvider,
		Admin:        new(u.IsAdmin()),
		Disabled:     &u.Disabled,
		Managed:      new(u.Kind == core.UserManaged),
		HasPin:       new(slices.ContainsFunc(creds, func(c core.Credential) bool { return c.Kind == core.CredentialPIN })),
		HasTotp:      new(slices.ContainsFunc(creds, func(c core.Credential) bool { return c.Kind == core.CredentialTOTP })),
		Passkeys:     passkeys,
		Restriction:  new(restrictions[u.Restriction]),
		Policy:       policy,
		PlaybackPreferences: userv1.PlaybackPreferences_builder{
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

// passkeyToProto converts a passkey credential, leaving out its key.
func passkeyToProto(c *core.Credential) *userv1.Passkey {
	var lastUse *timestamppb.Timestamp
	if c.LastUsedAt != nil {
		lastUse = timestamppb.New(*c.LastUsedAt)
	}
	return userv1.Passkey_builder{
		Id: new(c.ID.String()), Name: &c.Name, RpId: new(auth.PasskeyRPID(c.Data)),
		CreateTime: timestamppb.New(c.CreatedAt), LastUseTime: lastUse,
	}.Build()
}
