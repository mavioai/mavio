package subtitles

import "github.com/mavioai/mavio/libs/core"

// Scores weighs matched properties.
type Scores map[string]int

// EpisodeScores and MovieScores return Bazarr's default weights. A hash
// match is worth every other property but one point.
func EpisodeScores() Scores {
	return Scores{
		MatchHash: 359, MatchSeries: 160, MatchYear: 90, MatchSeason: 30, MatchEpisode: 30,
		MatchSource: 25, MatchReleaseGroup: 20, MatchAudioCodec: 1, MatchResolution: 1,
		MatchVideoCodec: 1, MatchHearingImpaired: 1, MatchStreamingService: 1,
	}
}

// MovieScores returns the default weights of movie subtitles.
func MovieScores() Scores {
	return Scores{
		MatchHash: 179, MatchTitle: 60, MatchYear: 40, MatchSource: 30, MatchEdition: 30,
		MatchReleaseGroup: 15, MatchAudioCodec: 1, MatchResolution: 1, MatchVideoCodec: 1,
		MatchHearingImpaired: 1, MatchStreamingService: 1,
	}
}

// ScoresFor returns the weights for episodes or movies.
func ScoresFor(episode bool) Scores {
	if episode {
		return EpisodeScores()
	}
	return MovieScores()
}

// Max returns the sum of the weights other than the hash: the score of a
// subtitle matching everything.
func (s Scores) Max() int {
	n := 0
	for k, v := range s {
		if k != MatchHash {
			n += v
		}
	}
	return n
}

// MaxScore returns the highest score of an episode or movie subtitle.
func MaxScore(episode bool) int { return ScoresFor(episode).Max() }

// MinScore returns the lowest score a subtitle needs to be downloaded
// automatically, from the percentage in the settings.
func MinScore(s *core.SubtitleSettings, episode bool) int {
	pct := s.MinimumScoreMovie
	if episode {
		pct = s.MinimumScore
	}
	return MaxScore(episode) * pct / 100
}

// ScoreSubject describes the subtitle being scored.
type ScoreSubject struct {
	// HashVerifiable is whether the provider's hash matches can be checked
	// against other properties.
	HashVerifiable  bool
	HearingImpaired bool
}

// Score computes a subtitle's score as Bazarr's compute_score: a trusted
// hash match replaces every other match, identifiers stand for the
// properties they imply, and a subtitle whose hearing-impaired flag is the
// wanted one (hi non-nil) gets the hearing_impaired match. It returns the
// score, the score without the hash, and the final matches; m is not
// modified.
func Score(m Matches, sub ScoreSubject, v *Video, hi *bool) (score, withoutHash int, final Matches) {
	scores := ScoresFor(v.Episode)
	orig := m.Clone()
	final = m.Clone()
	if final[MatchHash] {
		switch {
		case !sub.HashVerifiable:
			final = NewMatches(MatchHash)
		case !v.Episode || !v.Special:
			// Hashes are error-prone; trust one only when the essentials
			// match too. Specials' seasons and episodes tend to be wrong, so
			// their hashes are not checked.
			valid := final.Has(MatchVideoCodec, MatchSource)
			if v.Episode {
				valid = final.Has(MatchSeries, MatchSeason, MatchEpisode, MatchSource)
			}
			if valid {
				final = NewMatches(MatchHash)
			} else {
				delete(final, MatchHash)
			}
		}
	}

	eq := Matches{}
	if v.Episode {
		if final[MatchTitle] {
			eq.Add(NewMatches(MatchEpisode))
		}
		if final[MatchSeriesIMDbID] || final[MatchSeriesTVDBID] {
			eq.Add(NewMatches(MatchSeries, MatchYear))
		}
		if final[MatchIMDbID] {
			eq.Add(NewMatches(MatchSeries, MatchYear, MatchSeason, MatchEpisode))
		}
		if final[MatchTVDBID] {
			eq.Add(NewMatches(MatchSeries, MatchYear, MatchSeason, MatchEpisode, MatchTitle))
		}
		if v.Special && final.Has(MatchTitle, MatchSeries, MatchYear) {
			eq.Add(NewMatches(MatchSeason, MatchEpisode))
		}
	} else if final[MatchIMDbID] {
		eq.Add(NewMatches(MatchTitle, MatchYear))
	}
	final.Add(eq)

	if hi != nil && sub.HearingImpaired == *hi {
		final[MatchHearingImpaired] = true
		orig[MatchHearingImpaired] = true
	}

	for k := range final {
		score += scores[k]
	}
	orig.Add(eq)
	for k := range orig {
		if k != MatchHash {
			withoutHash += scores[k]
		}
	}
	return score, withoutHash, final
}
