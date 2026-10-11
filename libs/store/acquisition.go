package store

import (
	"context"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/customformat"
	"github.com/mavioai/mavio/libs/store/internal/ent/delayprofile"
	"github.com/mavioai/mavio/libs/store/internal/ent/qualitydefinition"
	"github.com/mavioai/mavio/libs/store/internal/ent/qualityprofile"
	"github.com/mavioai/mavio/libs/store/internal/ent/releaseprofile"
	"github.com/mavioai/mavio/libs/store/internal/ent/remotepathmapping"
	"github.com/mavioai/mavio/libs/store/internal/ent/wanted"
)

// The acquisition settings: quality definitions and profiles, custom
// formats, delay and release profiles, wanted items and remote path
// mappings. The tracked state (downloads, history, blocklist, pending
// releases) is in acquisition_state.go.

// validAcquisitionKind reports whether acquisition settings apply to k.
func validAcquisitionKind(k core.ItemKind) bool { return k == core.KindMovie || k == core.KindSeries }

type qualityDefinitions struct{ s *Store }

// QualityDefinitions returns the quality definitions.
func (s *Store) QualityDefinitions() core.QualityDefinitionRepository { return qualityDefinitions{s} }

func (r qualityDefinitions) List(ctx context.Context, kind core.ItemKind) ([]core.QualityDefinition, error) {
	rows, err := r.s.read.QualityDefinition.Query().Where(qualitydefinition.Kind(string(kind))).
		Order(ent.Asc(qualitydefinition.FieldQuality)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list quality definitions")
	}
	out := make([]core.QualityDefinition, len(rows))
	for i, d := range rows {
		out[i] = core.QualityDefinition{
			Kind: core.ItemKind(d.Kind), Quality: d.Quality, Title: d.Title,
			MinSize: d.MinSize, MaxSize: d.MaxSize, PreferredSize: d.PreferredSize,
		}
	}
	return out, nil
}

func (r qualityDefinitions) Replace(ctx context.Context, kind core.ItemKind, defs []core.QualityDefinition) error {
	if !validAcquisitionKind(kind) {
		return fmt.Errorf("%w: quality definitions of kind %q", core.ErrInvalid, kind)
	}
	for i := range defs {
		if defs[i].Kind == "" {
			defs[i].Kind = kind
		}
		if defs[i].Kind != kind {
			return fmt.Errorf("%w: quality definition %d of kind %q among those of %q", core.ErrInvalid, defs[i].Quality, defs[i].Kind, kind)
		}
		if err := defs[i].Validate(); err != nil {
			return err
		}
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		if _, err := tx.write.QualityDefinition.Delete().Where(qualitydefinition.Kind(string(kind))).Exec(ctx); err != nil {
			return mapErr(err, "replace quality definitions")
		}
		creates := make([]*ent.QualityDefinitionCreate, len(defs))
		for i, d := range defs {
			creates[i] = tx.write.QualityDefinition.Create().SetKind(string(kind)).SetQuality(d.Quality).SetTitle(d.Title).
				SetMinSize(d.MinSize).SetMaxSize(d.MaxSize).SetPreferredSize(d.PreferredSize)
		}
		return mapErr(tx.write.QualityDefinition.CreateBulk(creates...).Exec(ctx), "replace quality definitions")
	})
}

type qualityProfiles struct{ s *Store }

// QualityProfiles returns the quality profiles.
func (s *Store) QualityProfiles() core.QualityProfileRepository { return qualityProfiles{s} }

func (r qualityProfiles) Get(ctx context.Context, id core.ID) (core.QualityProfile, error) {
	p, err := r.s.read.QualityProfile.Get(ctx, id)
	if err != nil {
		return core.QualityProfile{}, mapErr(err, "get quality profile")
	}
	return toQualityProfile(p), nil
}

func (r qualityProfiles) List(ctx context.Context, kind core.ItemKind) ([]core.QualityProfile, error) {
	q := r.s.read.QualityProfile.Query()
	if kind != "" {
		q.Where(qualityprofile.Kind(string(kind)))
	}
	rows, err := q.Order(ent.Asc(qualityprofile.FieldName), ent.Asc(qualityprofile.FieldKind), ent.Asc(qualityprofile.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list quality profiles")
	}
	out := make([]core.QualityProfile, len(rows))
	for i, p := range rows {
		out[i] = toQualityProfile(p)
	}
	return out, nil
}

func (r qualityProfiles) Put(ctx context.Context, p *core.QualityProfile) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	err := r.s.write.QualityProfile.Create().
		SetID(p.ID).
		SetKind(string(p.Kind)).
		SetName(p.Name).
		SetUpgradeAllowed(p.UpgradeAllowed).
		SetCutoff(p.Cutoff).
		SetItems(p.Items).
		SetMinFormatScore(p.MinFormatScore).
		SetCutoffFormatScore(p.CutoffFormatScore).
		SetMinUpgradeFormatScore(p.MinUpgradeFormatScore).
		SetFormatScores(p.FormatScores).
		SetLanguage(p.Language).
		OnConflictColumns(qualityprofile.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put quality profile")
}

func (r qualityProfiles) Delete(ctx context.Context, id core.ID) error {
	return r.s.writeTx(ctx, func(tx *Store) error {
		used, err := tx.write.Wanted.Query().Where(wanted.QualityProfileID(id)).Exist(ctx)
		if err != nil {
			return mapErr(err, "delete quality profile")
		}
		if used {
			return fmt.Errorf("delete quality profile %s: %w: wanted items use it", id, core.ErrConflict)
		}
		return mapErr(tx.write.QualityProfile.DeleteOneID(id).Exec(ctx), "delete quality profile")
	})
}

func toQualityProfile(p *ent.QualityProfile) core.QualityProfile {
	return core.QualityProfile{
		ID: p.ID, Kind: core.ItemKind(p.Kind), Name: p.Name, UpgradeAllowed: p.UpgradeAllowed, Cutoff: p.Cutoff,
		Items: p.Items, MinFormatScore: p.MinFormatScore, CutoffFormatScore: p.CutoffFormatScore,
		MinUpgradeFormatScore: p.MinUpgradeFormatScore, FormatScores: p.FormatScores, Language: p.Language,
	}
}

type customFormats struct{ s *Store }

// CustomFormats returns the custom formats.
func (s *Store) CustomFormats() core.CustomFormatRepository { return customFormats{s} }

func (r customFormats) Get(ctx context.Context, id core.ID) (core.CustomFormat, error) {
	f, err := r.s.read.CustomFormat.Get(ctx, id)
	if err != nil {
		return core.CustomFormat{}, mapErr(err, "get custom format")
	}
	return toCustomFormat(f), nil
}

func (r customFormats) List(ctx context.Context) ([]core.CustomFormat, error) {
	rows, err := r.s.read.CustomFormat.Query().Order(ent.Asc(customformat.FieldName)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list custom formats")
	}
	out := make([]core.CustomFormat, len(rows))
	for i, f := range rows {
		out[i] = toCustomFormat(f)
	}
	return out, nil
}

func (r customFormats) Put(ctx context.Context, f *core.CustomFormat) error {
	if f.ID.IsZero() {
		f.ID = core.NewID()
	}
	if err := f.Validate(); err != nil {
		return err
	}
	err := r.s.write.CustomFormat.Create().
		SetID(f.ID).
		SetName(f.Name).
		SetIncludeWhenRenaming(f.IncludeWhenRenaming).
		SetSpecifications(f.Specifications).
		OnConflictColumns(customformat.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put custom format")
}

// Delete deletes the format and removes its scores from the profiles.
func (r customFormats) Delete(ctx context.Context, id core.ID) error {
	return r.s.writeTx(ctx, func(tx *Store) error {
		if err := tx.write.CustomFormat.DeleteOneID(id).Exec(ctx); err != nil {
			return mapErr(err, "delete custom format")
		}
		profiles, err := tx.write.QualityProfile.Query().All(ctx)
		if err != nil {
			return mapErr(err, "delete custom format")
		}
		left, err := tx.write.CustomFormat.Query().Exist(ctx)
		if err != nil {
			return mapErr(err, "delete custom format")
		}
		for _, p := range profiles {
			scores := slices.DeleteFunc(slices.Clone(p.FormatScores), func(s core.FormatScore) bool { return s.FormatID == id })
			if len(scores) == len(p.FormatScores) && left {
				continue
			}
			u := tx.write.QualityProfile.UpdateOneID(p.ID).SetFormatScores(scores)
			if !left {
				// Without formats, the format scores are reset, as
				// Radarr's QualityProfileService on CustomFormatDeletedEvent.
				u.SetMinFormatScore(0).SetCutoffFormatScore(0).SetMinUpgradeFormatScore(1)
			}
			if err := u.Exec(ctx); err != nil {
				return mapErr(err, "delete custom format")
			}
		}
		return nil
	})
}

func toCustomFormat(f *ent.CustomFormat) core.CustomFormat {
	return core.CustomFormat{ID: f.ID, Name: f.Name, IncludeWhenRenaming: f.IncludeWhenRenaming, Specifications: f.Specifications}
}

type delayProfiles struct{ s *Store }

// DelayProfiles returns the delay profiles.
func (s *Store) DelayProfiles() core.DelayProfileRepository { return delayProfiles{s} }

func (r delayProfiles) Get(ctx context.Context, id core.ID) (core.DelayProfile, error) {
	p, err := r.s.read.DelayProfile.Get(ctx, id)
	if err != nil {
		return core.DelayProfile{}, mapErr(err, "get delay profile")
	}
	return toDelayProfile(p), nil
}

func (r delayProfiles) List(ctx context.Context) ([]core.DelayProfile, error) {
	rows, err := r.s.read.DelayProfile.Query().Order(ent.Asc(delayprofile.FieldOrd), ent.Asc(delayprofile.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list delay profiles")
	}
	out := make([]core.DelayProfile, len(rows))
	for i, p := range rows {
		out[i] = toDelayProfile(p)
	}
	return out, nil
}

// Put stores the profile; a second default profile conflicts.
func (r delayProfiles) Put(ctx context.Context, p *core.DelayProfile) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	err := r.s.write.DelayProfile.Create().
		SetID(p.ID).
		SetIsDefault(p.Default).
		SetOrd(p.Order).
		SetEnableUsenet(p.EnableUsenet).
		SetEnableTorrent(p.EnableTorrent).
		SetPreferredProtocol(string(p.PreferredProtocol)).
		SetUsenetDelay(p.UsenetDelay).
		SetTorrentDelay(p.TorrentDelay).
		SetBypassIfHighestQuality(p.BypassIfHighestQuality).
		SetBypassIfAboveCustomFormatScore(p.BypassIfAboveCustomFormatScore).
		SetMinimumCustomFormatScore(p.MinimumCustomFormatScore).
		SetTags(p.Tags).
		OnConflictColumns(delayprofile.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put delay profile")
}

func (r delayProfiles) Delete(ctx context.Context, id core.ID) error {
	return r.s.writeTx(ctx, func(tx *Store) error {
		p, err := tx.write.DelayProfile.Get(ctx, id)
		if err != nil {
			return mapErr(err, "delete delay profile")
		}
		if p.IsDefault {
			return fmt.Errorf("delete delay profile %s: %w: it is the default", id, core.ErrConflict)
		}
		return mapErr(tx.write.DelayProfile.DeleteOneID(id).Exec(ctx), "delete delay profile")
	})
}

func toDelayProfile(p *ent.DelayProfile) core.DelayProfile {
	return core.DelayProfile{
		ID: p.ID, Default: p.IsDefault, Order: p.Ord, EnableUsenet: p.EnableUsenet, EnableTorrent: p.EnableTorrent,
		PreferredProtocol: core.Protocol(p.PreferredProtocol), UsenetDelay: p.UsenetDelay, TorrentDelay: p.TorrentDelay,
		BypassIfHighestQuality: p.BypassIfHighestQuality, BypassIfAboveCustomFormatScore: p.BypassIfAboveCustomFormatScore,
		MinimumCustomFormatScore: p.MinimumCustomFormatScore, Tags: p.Tags,
	}
}

type releaseProfiles struct{ s *Store }

// ReleaseProfiles returns the release profiles.
func (s *Store) ReleaseProfiles() core.ReleaseProfileRepository { return releaseProfiles{s} }

func (r releaseProfiles) Get(ctx context.Context, id core.ID) (core.ReleaseProfile, error) {
	p, err := r.s.read.ReleaseProfile.Get(ctx, id)
	if err != nil {
		return core.ReleaseProfile{}, mapErr(err, "get release profile")
	}
	return toReleaseProfile(p), nil
}

func (r releaseProfiles) List(ctx context.Context) ([]core.ReleaseProfile, error) {
	rows, err := r.s.read.ReleaseProfile.Query().Order(ent.Asc(releaseprofile.FieldName), ent.Asc(releaseprofile.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list release profiles")
	}
	out := make([]core.ReleaseProfile, len(rows))
	for i, p := range rows {
		out[i] = toReleaseProfile(p)
	}
	return out, nil
}

func (r releaseProfiles) Put(ctx context.Context, p *core.ReleaseProfile) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	err := r.s.write.ReleaseProfile.Create().
		SetID(p.ID).
		SetName(p.Name).
		SetEnabled(p.Enabled).
		SetRequired(p.Required).
		SetIgnored(p.Ignored).
		SetIndexerID(p.IndexerID).
		SetTags(p.Tags).
		OnConflictColumns(releaseprofile.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put release profile")
}

func (r releaseProfiles) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.ReleaseProfile.DeleteOneID(id).Exec(ctx), "delete release profile")
}

func toReleaseProfile(p *ent.ReleaseProfile) core.ReleaseProfile {
	return core.ReleaseProfile{
		ID: p.ID, Name: p.Name, Enabled: p.Enabled, Required: p.Required, Ignored: p.Ignored,
		IndexerID: p.IndexerID, Tags: p.Tags,
	}
}

type wantedItems struct{ s *Store }

// Wanted returns how monitored items are acquired.
func (s *Store) Wanted() core.WantedRepository { return wantedItems{s} }

func (r wantedItems) Get(ctx context.Context, itemID core.ID) (core.Wanted, error) {
	w, err := r.s.read.Wanted.Query().Where(wanted.ItemID(itemID)).Only(ctx)
	if err != nil {
		return core.Wanted{}, mapErr(err, "get wanted")
	}
	return toWanted(w), nil
}

func (r wantedItems) List(ctx context.Context, itemIDs ...core.ID) ([]core.Wanted, error) {
	q := r.s.read.Wanted.Query()
	if len(itemIDs) > 0 {
		q.Where(wanted.ItemIDIn(itemIDs...))
	}
	rows, err := q.Order(ent.Asc(wanted.FieldAddedAt), ent.Asc(wanted.FieldItemID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list wanted")
	}
	out := make([]core.Wanted, len(rows))
	for i, w := range rows {
		out[i] = toWanted(w)
	}
	return out, nil
}

// Put stores the item's wanted state; a zero AddedAt keeps the stored one,
// or is now for an item not wanted yet.
func (r wantedItems) Put(ctx context.Context, w *core.Wanted) error {
	if err := w.Validate(); err != nil {
		return err
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		old, err := tx.write.Wanted.Query().Where(wanted.ItemID(w.ItemID)).Only(ctx)
		switch {
		case ent.IsNotFound(err):
			w.AddedAt = orNow(w.AddedAt).UTC()
			c := tx.write.Wanted.Create().
				SetItemID(w.ItemID).
				SetQualityProfileID(w.QualityProfileID).
				SetMinimumAvailability(string(w.MinimumAvailability)).
				SetMonitorNewItems(string(w.MonitorNewItems)).
				SetSeasonFolder(w.SeasonFolder).
				SetTags(w.Tags).
				SetAddedAt(w.AddedAt).
				SetNillableLastSearchedAt(nonZero(w.LastSearchedAt))
			return mapErr(c.Exec(ctx), "put wanted")
		case err != nil:
			return mapErr(err, "put wanted")
		}
		if w.AddedAt.IsZero() {
			w.AddedAt = old.AddedAt.UTC()
		}
		u := tx.write.Wanted.UpdateOne(old).
			SetQualityProfileID(w.QualityProfileID).
			SetMinimumAvailability(string(w.MinimumAvailability)).
			SetMonitorNewItems(string(w.MonitorNewItems)).
			SetSeasonFolder(w.SeasonFolder).
			SetTags(w.Tags).
			SetAddedAt(w.AddedAt)
		if w.LastSearchedAt.IsZero() {
			u.ClearLastSearchedAt()
		} else {
			u.SetLastSearchedAt(w.LastSearchedAt)
		}
		return mapErr(u.Exec(ctx), "put wanted")
	})
}

func (r wantedItems) Delete(ctx context.Context, itemID core.ID) error {
	n, err := r.s.write.Wanted.Delete().Where(wanted.ItemID(itemID)).Exec(ctx)
	if err != nil {
		return mapErr(err, "delete wanted")
	}
	if n == 0 {
		return fmt.Errorf("delete wanted %s: %w", itemID, core.ErrNotFound)
	}
	return nil
}

func toWanted(w *ent.Wanted) core.Wanted {
	return core.Wanted{
		ItemID: w.ItemID, QualityProfileID: w.QualityProfileID, MinimumAvailability: core.MovieAvailability(w.MinimumAvailability),
		MonitorNewItems: core.NewItemMonitor(w.MonitorNewItems), SeasonFolder: w.SeasonFolder,
		Tags: w.Tags, AddedAt: w.AddedAt.UTC(), LastSearchedAt: zeroIfNil(w.LastSearchedAt),
	}
}

type remotePathMappings struct{ s *Store }

// RemotePathMappings returns the remote path mappings.
func (s *Store) RemotePathMappings() core.RemotePathMappingRepository { return remotePathMappings{s} }

func (r remotePathMappings) List(ctx context.Context) ([]core.RemotePathMapping, error) {
	rows, err := r.s.read.RemotePathMapping.Query().
		Order(ent.Asc(remotepathmapping.FieldHost), ent.Asc(remotepathmapping.FieldRemotePath)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list remote path mappings")
	}
	out := make([]core.RemotePathMapping, len(rows))
	for i, m := range rows {
		out[i] = core.RemotePathMapping{ID: m.ID, Host: m.Host, RemotePath: m.RemotePath, LocalPath: m.LocalPath}
	}
	return out, nil
}

func (r remotePathMappings) Put(ctx context.Context, m *core.RemotePathMapping) error {
	if m.ID.IsZero() {
		m.ID = core.NewID()
	}
	if err := m.Validate(); err != nil {
		return err
	}
	err := r.s.write.RemotePathMapping.Create().
		SetID(m.ID).
		SetHost(m.Host).
		SetRemotePath(m.RemotePath).
		SetLocalPath(m.LocalPath).
		OnConflictColumns(remotepathmapping.FieldID).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put remote path mapping")
}

func (r remotePathMappings) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.RemotePathMapping.DeleteOneID(id).Exec(ctx), "delete remote path mapping")
}
