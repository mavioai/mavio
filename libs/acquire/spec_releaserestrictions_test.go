package acquire

import "testing"

// TestReleaseRestrictionsSpec ports Radarr's
// ReleaseRestrictionsSpecificationFixture at the level of the
// specification: its cases of required and ignored terms, case
// insensitivity and Perl regular expressions (should_ignore_case_*,
// should_match_perl_regex and the combined restricted and required words)
// match the terms through TermMatcher.CheckTerms and are ported in
// TestCheckTerms. Here are its cases that check the specification's own
// part — which profiles apply and the rejection reasons — and the
// selection Radarr's mocked ReleaseProfileService.EnabledForTags makes.
func TestReleaseRestrictionsSpec(t *testing.T) {
	const title = "Dexter.S08E01.EDITED.WEBRip.x264-KYR"
	tests := []struct {
		name     string
		profiles []ReleaseProfile
		tags     []string
		want     Reason // "" when accepted
	}{
		{"should_be_true_when_restrictions_are_empty", nil, nil, ""},
		{
			"should_be_true_when_title_contains_one_required_term",
			[]ReleaseProfile{{Enabled: true, Required: []string{"WEBRip"}}},
			nil, "",
		},
		{
			"should_be_false_when_title_does_not_contain_any_required_terms",
			[]ReleaseProfile{{Enabled: true, Required: []string{"doesnt", "exist"}}},
			nil, ReasonMustContainMissing,
		},
		{
			"should_be_true_when_title_does_not_contain_any_ignored_terms",
			[]ReleaseProfile{{Enabled: true, Ignored: []string{"ignored"}}},
			nil, "",
		},
		{
			"should_be_false_when_title_contains_one_anded_ignored_terms",
			[]ReleaseProfile{{Enabled: true, Ignored: []string{"edited"}}},
			nil, ReasonMustNotContainPresent,
		},
		// Not in the fixture, which mocks EnabledForTags: the profiles that
		// do not apply are left out.
		{
			"disabled profile",
			[]ReleaseProfile{{Enabled: false, Required: []string{"doesnt"}}},
			nil, "",
		},
		{
			"profile of another indexer",
			[]ReleaseProfile{{Enabled: true, IndexerID: ixID(2), Required: []string{"doesnt"}}},
			nil, "",
		},
		{
			"profile of the release's indexer",
			[]ReleaseProfile{{Enabled: true, IndexerID: ixID(1), Required: []string{"doesnt"}}},
			nil, ReasonMustContainMissing,
		},
		{
			"profile of other tags",
			[]ReleaseProfile{{Enabled: true, Tags: []string{"5"}, Ignored: []string{"edited"}}},
			[]string{"6"},
			"",
		},
		{
			"profile of the movie's tags",
			[]ReleaseProfile{{Enabled: true, Tags: []string{"5"}, Ignored: []string{"edited"}}},
			[]string{"5", "6"},
			ReasonMustNotContainPresent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.ReleaseProfiles = tt.profiles
			c := spCandidate()
			c.Release.Title = title
			c.Target.Tags = tt.tags
			var got Reason
			if r := check(t, ReleaseRestrictions{}, e, c, nil); r != nil {
				got = r.Reason
			}
			if got != tt.want {
				t.Errorf("ReleaseRestrictions reason = %q, want = %q", got, tt.want)
			}
		})
	}
}
