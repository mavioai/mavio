package store

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/episodestate"
)

type episodeStates struct{ s *Store }

// EpisodeStates returns the states of episodes of series.
func (s *Store) EpisodeStates() core.EpisodeStateRepository { return episodeStates{s} }

func (r episodeStates) List(ctx context.Context, seriesID core.ID) ([]core.EpisodeState, error) {
	rows, err := r.s.read.EpisodeState.Query().
		Where(episodestate.SeriesID(seriesID)).
		Order(ent.Asc(episodestate.FieldSeason), ent.Asc(episodestate.FieldEpisode)).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list episode states")
	}
	out := make([]core.EpisodeState, len(rows))
	for i, row := range rows {
		out[i] = core.EpisodeState{
			SeriesID:      row.SeriesID,
			EpisodeNumber: core.EpisodeNumber{Season: row.Season, Episode: row.Episode},
			SceneSeason:   row.SceneSeason, SceneEpisode: row.SceneEpisode, SceneAbsolute: row.SceneAbsolute,
			UnverifiedSceneNumbering: row.UnverifiedSceneNumbering,
			LastSearchedAt:           zeroIfNil(row.LastSearchedAt),
		}
	}
	return out, nil
}

func checkEpisodeNumber(seriesID core.ID, n core.EpisodeNumber) error {
	if seriesID.IsZero() || n.Season < 0 || n.Episode < 0 {
		return fmt.Errorf("%w: episode state requires a series and a season and episode of at least 0", core.ErrInvalid)
	}
	return nil
}

func (r episodeStates) PutSceneNumbering(ctx context.Context, seriesID core.ID, states []core.EpisodeState) error {
	for i := range states {
		if err := checkEpisodeNumber(seriesID, states[i].EpisodeNumber); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		err := tx.write.EpisodeState.Update().
			Where(episodestate.SeriesID(seriesID)).
			ClearSceneSeason().ClearSceneEpisode().ClearSceneAbsolute().
			SetUnverifiedSceneNumbering(false).
			Exec(ctx)
		if err != nil {
			return mapErr(err, "clear scene numbering")
		}
		for chunk := range slices.Chunk(states, upsertBatch) {
			creates := make([]*ent.EpisodeStateCreate, len(chunk))
			for i, s := range chunk {
				creates[i] = tx.write.EpisodeState.Create().
					SetSeriesID(seriesID).
					SetSeason(s.Season).
					SetEpisode(s.Episode).
					SetNillableSceneSeason(s.SceneSeason).
					SetNillableSceneEpisode(s.SceneEpisode).
					SetNillableSceneAbsolute(s.SceneAbsolute).
					SetUnverifiedSceneNumbering(s.UnverifiedSceneNumbering)
			}
			err := tx.write.EpisodeState.CreateBulk(creates...).
				OnConflictColumns(episodestate.FieldSeriesID, episodestate.FieldSeason, episodestate.FieldEpisode).
				Update(func(u *ent.EpisodeStateUpsert) {
					u.UpdateSceneSeason().UpdateSceneEpisode().UpdateSceneAbsolute().UpdateUnverifiedSceneNumbering()
				}).
				Exec(ctx)
			if err != nil {
				return mapErr(err, "put scene numbering")
			}
		}
		// States left with neither scene numbering nor a search time
		// keep nothing.
		_, err = tx.write.EpisodeState.Delete().
			Where(
				episodestate.SeriesID(seriesID), episodestate.SceneSeasonIsNil(), episodestate.SceneEpisodeIsNil(),
				episodestate.SceneAbsoluteIsNil(), episodestate.UnverifiedSceneNumbering(false), episodestate.LastSearchedAtIsNil(),
			).
			Exec(ctx)
		return mapErr(err, "put scene numbering")
	})
}

func (r episodeStates) Searched(ctx context.Context, seriesID core.ID, at time.Time, episodes ...core.EpisodeNumber) error {
	for _, n := range episodes {
		if err := checkEpisodeNumber(seriesID, n); err != nil {
			return err
		}
	}
	at = at.UTC()
	return r.s.writeTx(ctx, func(tx *Store) error {
		for chunk := range slices.Chunk(episodes, upsertBatch) {
			creates := make([]*ent.EpisodeStateCreate, len(chunk))
			for i, n := range chunk {
				creates[i] = tx.write.EpisodeState.Create().
					SetSeriesID(seriesID).
					SetSeason(n.Season).
					SetEpisode(n.Episode).
					SetLastSearchedAt(at)
			}
			err := tx.write.EpisodeState.CreateBulk(creates...).
				OnConflictColumns(episodestate.FieldSeriesID, episodestate.FieldSeason, episodestate.FieldEpisode).
				Update(func(u *ent.EpisodeStateUpsert) { u.UpdateLastSearchedAt() }).
				Exec(ctx)
			if err != nil {
				return mapErr(err, "record episode searches")
			}
		}
		return nil
	})
}
