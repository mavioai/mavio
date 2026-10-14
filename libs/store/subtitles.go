package store

import (
	"context"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/item"
	"github.com/mavioai/mavio/libs/store/internal/ent/library"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitleblocklistentry"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitlehistoryevent"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitleprofile"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitleprofileassignment"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitlesearchattempt"
	"github.com/mavioai/mavio/libs/store/internal/ent/subtitlewanted"
)

type subtitleProfiles struct{ s *Store }

// SubtitleProfiles returns the subtitle profiles.
func (s *Store) SubtitleProfiles() core.SubtitleProfileRepository { return subtitleProfiles{s} }

func (r subtitleProfiles) Get(ctx context.Context, id core.ID) (core.SubtitleProfile, error) {
	p, err := r.s.read.SubtitleProfile.Get(ctx, id)
	if err != nil {
		return core.SubtitleProfile{}, mapErr(err, "get subtitle profile")
	}
	return toSubtitleProfile(p), nil
}

func (r subtitleProfiles) List(ctx context.Context) ([]core.SubtitleProfile, error) {
	rows, err := r.s.read.SubtitleProfile.Query().Order(ent.Asc(subtitleprofile.FieldName)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list subtitle profiles")
	}
	out := make([]core.SubtitleProfile, len(rows))
	for i, p := range rows {
		out[i] = toSubtitleProfile(p)
	}
	return out, nil
}

func (r subtitleProfiles) Put(ctx context.Context, p *core.SubtitleProfile) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	err := r.s.write.SubtitleProfile.Create().
		SetID(p.ID).
		SetName(p.Name).
		SetItems(p.Items).
		SetCutoff(p.Cutoff).
		SetMustContain(p.MustContain).
		SetMustNotContain(p.MustNotContain).
		SetOriginalFormat(p.OriginalFormat).
		SetTag(p.Tag).
		OnConflictColumns(subtitleprofile.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put subtitle profile")
}

func (r subtitleProfiles) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.SubtitleProfile.DeleteOneID(id).Exec(ctx), "delete subtitle profile")
}

func toSubtitleProfile(p *ent.SubtitleProfile) core.SubtitleProfile {
	return core.SubtitleProfile{
		ID: p.ID, Name: p.Name, Items: p.Items, Cutoff: p.Cutoff, MustContain: p.MustContain,
		MustNotContain: p.MustNotContain, OriginalFormat: p.OriginalFormat, Tag: p.Tag,
	}
}

type subtitleProfileAssignments struct{ s *Store }

// SubtitleProfileAssignments returns the assignments of subtitle profiles.
func (s *Store) SubtitleProfileAssignments() core.SubtitleProfileAssignmentRepository {
	return subtitleProfileAssignments{s}
}

// targets selects the assignments of targets, libraries or items.
func targets(ids ...core.ID) predicate.SubtitleProfileAssignment {
	return subtitleprofileassignment.Or(
		subtitleprofileassignment.LibraryIDIn(ids...), subtitleprofileassignment.ItemIDIn(ids...))
}

func (r subtitleProfileAssignments) Get(ctx context.Context, targetID core.ID) (core.SubtitleProfileAssignment, error) {
	a, err := r.s.read.SubtitleProfileAssignment.Query().Where(targets(targetID)).Only(ctx)
	if err != nil {
		return core.SubtitleProfileAssignment{}, mapErr(err, "get subtitle profile assignment")
	}
	return toSubtitleProfileAssignment(a), nil
}

func (r subtitleProfileAssignments) List(ctx context.Context, targetIDs ...core.ID) ([]core.SubtitleProfileAssignment, error) {
	q := r.s.read.SubtitleProfileAssignment.Query()
	if len(targetIDs) > 0 {
		q.Where(targets(targetIDs...))
	}
	rows, err := q.Order(ent.Asc(subtitleprofileassignment.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list subtitle profile assignments")
	}
	out := make([]core.SubtitleProfileAssignment, len(rows))
	for i, a := range rows {
		out[i] = toSubtitleProfileAssignment(a)
	}
	return out, nil
}

func (r subtitleProfileAssignments) Put(ctx context.Context, a core.SubtitleProfileAssignment) error {
	if a.TargetID.IsZero() {
		return fmt.Errorf("%w: subtitle profile assignment requires a target", core.ErrInvalid)
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		c := tx.write.SubtitleProfileAssignment.Create()
		switch lib, err := tx.write.Library.Query().Where(library.ID(a.TargetID)).Exist(ctx); {
		case err != nil:
			return mapErr(err, "put subtitle profile assignment")
		case lib:
			c.SetLibraryID(a.TargetID)
		default:
			found, err := tx.write.Item.Query().Where(item.ID(a.TargetID)).Exist(ctx)
			if err != nil {
				return mapErr(err, "put subtitle profile assignment")
			}
			if !found {
				return fmt.Errorf("put subtitle profile assignment: target %s: %w", a.TargetID, core.ErrNotFound)
			}
			c.SetItemID(a.TargetID)
		}
		if !a.ProfileID.IsZero() {
			ok, err := tx.write.SubtitleProfile.Query().Where(subtitleprofile.ID(a.ProfileID)).Exist(ctx)
			if err != nil {
				return mapErr(err, "put subtitle profile assignment")
			}
			if !ok {
				return fmt.Errorf("put subtitle profile assignment: profile %s: %w", a.ProfileID, core.ErrNotFound)
			}
			c.SetProfileID(a.ProfileID)
		}
		if _, err := tx.write.SubtitleProfileAssignment.Delete().Where(targets(a.TargetID)).Exec(ctx); err != nil {
			return mapErr(err, "put subtitle profile assignment")
		}
		return mapErr(c.Exec(ctx), "put subtitle profile assignment")
	})
}

func (r subtitleProfileAssignments) Delete(ctx context.Context, targetID core.ID) error {
	_, err := r.s.write.SubtitleProfileAssignment.Delete().Where(targets(targetID)).Exec(ctx)
	return mapErr(err, "delete subtitle profile assignment")
}

func toSubtitleProfileAssignment(a *ent.SubtitleProfileAssignment) core.SubtitleProfileAssignment {
	out := core.SubtitleProfileAssignment{ProfileID: zeroIDIfNil(a.ProfileID)}
	if a.LibraryID != nil {
		out.TargetID = *a.LibraryID
	} else {
		out.TargetID = zeroIDIfNil(a.ItemID)
	}
	return out
}

func zeroIDIfNil(id *core.ID) core.ID {
	if id == nil {
		return core.ID{}
	}
	return *id
}

type subtitleHistory struct{ s *Store }

// SubtitleHistory returns the subtitle history.
func (s *Store) SubtitleHistory() core.SubtitleHistoryRepository { return subtitleHistory{s} }

func (r subtitleHistory) Add(ctx context.Context, events ...core.SubtitleHistoryEvent) error {
	for i := range events {
		if events[i].ID.IsZero() {
			events[i].ID = core.NewID()
		}
		if err := events[i].Validate(); err != nil {
			return err
		}
	}
	if len(events) == 0 {
		return nil
	}
	creates := make([]*ent.SubtitleHistoryEventCreate, len(events))
	for i, h := range events {
		creates[i] = r.s.write.SubtitleHistoryEvent.Create().
			SetID(h.ID).
			SetAction(string(h.Action)).
			SetItemID(h.ItemID).
			SetNillableParentID(nonZeroID(h.ParentID)).
			SetDate(h.Date).
			SetLanguage(h.Language.Language).
			SetForced(h.Language.Forced).
			SetHearingImpaired(h.Language.HearingImpaired).
			SetProvider(h.Provider).
			SetSubtitleID(h.SubtitleID).
			SetScore(h.Score).
			SetMaxScore(h.MaxScore).
			SetPath(h.Path).
			SetVideoPath(h.VideoPath).
			SetDescription(h.Description).
			SetMatched(h.Matched).
			SetNotMatched(h.NotMatched).
			SetNillableUpgradedFrom(nonZeroID(h.UpgradedFrom))
	}
	return mapErr(r.s.write.SubtitleHistoryEvent.CreateBulk(creates...).Exec(ctx), "add subtitle history")
}

func (r subtitleHistory) List(ctx context.Context, q core.SubtitleHistoryQuery) (core.Page[core.SubtitleHistoryEvent], error) {
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.SubtitleHistoryEvent]{}, err
	}
	var where []predicate.SubtitleHistoryEvent
	if len(q.ItemIDs) > 0 {
		where = append(where, subtitlehistoryevent.Or(
			subtitlehistoryevent.ItemIDIn(q.ItemIDs...), subtitlehistoryevent.ParentIDIn(q.ItemIDs...)))
	}
	if len(q.Actions) > 0 {
		actions := make([]string, len(q.Actions))
		for i, a := range q.Actions {
			actions[i] = string(a)
		}
		where = append(where, subtitlehistoryevent.ActionIn(actions...))
	}
	if !q.Since.IsZero() {
		where = append(where, subtitlehistoryevent.DateGTE(q.Since))
	}
	query := r.s.read.SubtitleHistoryEvent.Query().Where(where...)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.SubtitleHistoryEvent]{}, mapErr(err, "count subtitle history")
	}
	rows, err := query.Order(ent.Desc(subtitlehistoryevent.FieldDate), ent.Desc(subtitlehistoryevent.FieldID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.SubtitleHistoryEvent]{}, mapErr(err, "list subtitle history")
	}
	out := core.Page[core.SubtitleHistoryEvent]{Total: total, Items: make([]core.SubtitleHistoryEvent, len(rows))}
	for i, h := range rows {
		out.Items[i] = core.SubtitleHistoryEvent{
			ID: h.ID, Action: core.SubtitleAction(h.Action), ItemID: h.ItemID, ParentID: zeroIDIfNil(h.ParentID),
			Date: h.Date.UTC(), Language: subtitleLanguage(h.Language, h.Forced, h.HearingImpaired),
			Provider: h.Provider, SubtitleID: h.SubtitleID, Score: h.Score, MaxScore: h.MaxScore,
			Path: h.Path, VideoPath: h.VideoPath, Description: h.Description, Matched: h.Matched,
			NotMatched: h.NotMatched, UpgradedFrom: zeroIDIfNil(h.UpgradedFrom),
		}
	}
	return out, nil
}

func (r subtitleHistory) Purge(ctx context.Context, before time.Time) (int, error) {
	n, err := r.s.write.SubtitleHistoryEvent.Delete().Where(subtitlehistoryevent.DateLT(before)).Exec(ctx)
	return n, mapErr(err, "purge subtitle history")
}

func subtitleLanguage(lang string, forced, hi bool) core.SubtitleLanguage {
	return core.SubtitleLanguage{Language: lang, Forced: forced, HearingImpaired: hi}
}

type subtitleBlocklist struct{ s *Store }

// SubtitleBlocklist returns the blocklisted subtitles.
func (s *Store) SubtitleBlocklist() core.SubtitleBlocklistRepository { return subtitleBlocklist{s} }

func (r subtitleBlocklist) Add(ctx context.Context, b *core.SubtitleBlocklistEntry) error {
	if b.ID.IsZero() {
		b.ID = core.NewID()
	}
	if err := b.Validate(); err != nil {
		return err
	}
	err := r.s.write.SubtitleBlocklistEntry.Create().
		SetID(b.ID).
		SetItemID(b.ItemID).
		SetNillableParentID(nonZeroID(b.ParentID)).
		SetProvider(b.Provider).
		SetSubtitleID(b.SubtitleID).
		SetLanguage(b.Language.Language).
		SetForced(b.Language.Forced).
		SetHearingImpaired(b.Language.HearingImpaired).
		SetDate(b.Date).
		SetMessage(b.Message).
		Exec(ctx)
	return mapErr(err, "add subtitle blocklist entry")
}

func (r subtitleBlocklist) List(ctx context.Context, itemID core.ID, limit, offset int) (core.Page[core.SubtitleBlocklistEntry], error) {
	size, err := pageSize(limit, offset)
	if err != nil {
		return core.Page[core.SubtitleBlocklistEntry]{}, err
	}
	query := r.s.read.SubtitleBlocklistEntry.Query()
	if !itemID.IsZero() {
		query.Where(subtitleblocklistentry.ItemID(itemID))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.SubtitleBlocklistEntry]{}, mapErr(err, "count subtitle blocklist")
	}
	rows, err := query.Order(ent.Desc(subtitleblocklistentry.FieldDate), ent.Desc(subtitleblocklistentry.FieldID)).
		Limit(size).Offset(offset).All(ctx)
	if err != nil {
		return core.Page[core.SubtitleBlocklistEntry]{}, mapErr(err, "list subtitle blocklist")
	}
	out := core.Page[core.SubtitleBlocklistEntry]{Total: total, Items: make([]core.SubtitleBlocklistEntry, len(rows))}
	for i, b := range rows {
		out.Items[i] = core.SubtitleBlocklistEntry{
			ID: b.ID, ItemID: b.ItemID, ParentID: zeroIDIfNil(b.ParentID), Provider: b.Provider,
			SubtitleID: b.SubtitleID, Language: subtitleLanguage(b.Language, b.Forced, b.HearingImpaired),
			Date: b.Date.UTC(), Message: b.Message,
		}
	}
	return out, nil
}

func (r subtitleBlocklist) Delete(ctx context.Context, ids ...core.ID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.s.write.SubtitleBlocklistEntry.Delete().Where(subtitleblocklistentry.IDIn(ids...)).Exec(ctx)
	return mapErr(err, "delete subtitle blocklist entries")
}

func (r subtitleBlocklist) Clear(ctx context.Context) error {
	_, err := r.s.write.SubtitleBlocklistEntry.Delete().Exec(ctx)
	return mapErr(err, "clear subtitle blocklist")
}

type subtitleSearchAttempts struct{ s *Store }

// SubtitleSearchAttempts returns the failed subtitle searches.
func (s *Store) SubtitleSearchAttempts() core.SubtitleSearchAttemptRepository {
	return subtitleSearchAttempts{s}
}

func (r subtitleSearchAttempts) List(ctx context.Context, itemID core.ID) ([]core.SubtitleSearchAttempt, error) {
	rows, err := r.s.read.SubtitleSearchAttempt.Query().
		Where(subtitlesearchattempt.ItemID(itemID)).
		Order(ent.Asc(subtitlesearchattempt.FieldLanguage), ent.Asc(subtitlesearchattempt.FieldForced),
			ent.Asc(subtitlesearchattempt.FieldHearingImpaired)).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list subtitle search attempts")
	}
	out := make([]core.SubtitleSearchAttempt, len(rows))
	for i, a := range rows {
		out[i] = core.SubtitleSearchAttempt{
			ItemID: a.ItemID, Language: subtitleLanguage(a.Language, a.Forced, a.HearingImpaired),
			First: a.First.UTC(), Last: a.Last.UTC(),
		}
	}
	return out, nil
}

func (r subtitleSearchAttempts) Record(ctx context.Context, itemID core.ID, lang core.SubtitleLanguage, at time.Time) error {
	if itemID.IsZero() || at.IsZero() {
		return fmt.Errorf("%w: subtitle search attempt requires an item and a time", core.ErrInvalid)
	}
	if err := lang.Validate(); err != nil {
		return err
	}
	err := r.s.write.SubtitleSearchAttempt.Create().
		SetItemID(itemID).
		SetLanguage(lang.Language).
		SetForced(lang.Forced).
		SetHearingImpaired(lang.HearingImpaired).
		SetFirst(at).
		SetLast(at).
		OnConflictColumns(subtitlesearchattempt.FieldItemID, subtitlesearchattempt.FieldLanguage,
			subtitlesearchattempt.FieldForced, subtitlesearchattempt.FieldHearingImpaired).
		UpdateLast().
		Exec(ctx)
	return mapErr(err, "record subtitle search attempt")
}

type subtitleWanted struct{ s *Store }

// SubtitleWanted returns the movies and episodes missing subtitles.
func (s *Store) SubtitleWanted() core.SubtitleWantedRepository { return subtitleWanted{s} }

func (r subtitleWanted) Put(ctx context.Context, w core.SubtitleWanted) error {
	if len(w.Missing) == 0 {
		return r.Delete(ctx, w.ItemID)
	}
	if err := w.Validate(); err != nil {
		return err
	}
	err := r.s.write.SubtitleWanted.Create().
		SetID(core.NewID()).
		SetItemID(w.ItemID).
		SetKind(string(w.Kind)).
		SetLibraryID(w.LibraryID).
		SetNillableParentID(nonZeroID(w.ParentID)).
		SetPath(w.Path).
		SetMissing(w.Missing).
		SetUpdated(w.Updated).
		OnConflictColumns(subtitlewanted.FieldItemID).
		Update(func(u *ent.SubtitleWantedUpsert) {
			u.UpdateKind().UpdateLibraryID().UpdateParentID().UpdatePath().UpdateMissing().UpdateUpdated()
		}).
		Exec(ctx)
	return mapErr(err, "put wanted subtitles")
}

func (r subtitleWanted) Delete(ctx context.Context, itemIDs ...core.ID) error {
	if len(itemIDs) == 0 {
		return nil
	}
	_, err := r.s.write.SubtitleWanted.Delete().Where(subtitlewanted.ItemIDIn(itemIDs...)).Exec(ctx)
	return mapErr(err, "delete wanted subtitles")
}

func (r subtitleWanted) List(ctx context.Context, q core.SubtitleWantedQuery) (core.Page[core.SubtitleWanted], error) {
	limit, err := pageSize(q.Limit, q.Offset)
	if err != nil {
		return core.Page[core.SubtitleWanted]{}, err
	}
	query := r.s.read.SubtitleWanted.Query().Where(
		subtitlewanted.Kind(string(q.Kind)), subtitlewanted.HasItemWith(item.MissingSinceIsNil()))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return core.Page[core.SubtitleWanted]{}, mapErr(err, "count wanted subtitles")
	}
	rows, err := query.Order(ent.Asc(subtitlewanted.FieldLibraryID), ent.Asc(subtitlewanted.FieldPath), ent.Asc(subtitlewanted.FieldItemID)).
		Limit(limit).Offset(q.Offset).All(ctx)
	if err != nil {
		return core.Page[core.SubtitleWanted]{}, mapErr(err, "list wanted subtitles")
	}
	out := core.Page[core.SubtitleWanted]{Total: total, Items: make([]core.SubtitleWanted, len(rows))}
	for i, w := range rows {
		out.Items[i] = core.SubtitleWanted{
			ItemID: w.ItemID, Kind: core.ItemKind(w.Kind), LibraryID: w.LibraryID, ParentID: zeroIDIfNil(w.ParentID),
			Path: w.Path, Missing: w.Missing, Updated: w.Updated.UTC(),
		}
	}
	return out, nil
}
