package acquisition

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// run is what one search, sync or queue check decides with: the
// decision environment built from the store, and the movies it read.
type run struct {
	rt       *Runtime
	settings core.AcquisitionSettings
	env      *acquire.Env
	engine   *acquire.Engine

	indexers map[string]releases.Indexer
	// indexerList holds the indexers in the order ListIndexers gives.
	indexerList []releases.Indexer
	profiles    map[core.ID]*acquire.QualityProfile
	libs        map[core.ID]core.Library
	movies      map[core.ID]*movie
	refs        []acquire.MovieRef
	refsRead    bool

	// senv and sengine decide on episode releases.
	senv    *series.Env
	sengine *series.Engine
	shows   map[core.ID]*show
	// showList is every wanted series once showsRead.
	showList  []*show
	showsRead bool
	parser    *series.Parsing
	// holders are the items holding the episodes multi-episode files
	// cover beyond their first.
	holders map[core.ID]core.ID
	// sceneMappings are the scene mappings, read with the first show.
	sceneMappings *series.SceneMappings
}

// newRun builds the decision environment from the store.
func (r *Runtime) newRun(ctx context.Context) (*run, error) {
	s := r.settings()
	cfg, err := acquire.ConfigFromCore(s)
	if err != nil {
		return nil, fmt.Errorf("acquisition settings: %w", err)
	}
	stored, err := r.Store.QualityDefinitions().List(ctx, core.KindMovie)
	if err != nil {
		return nil, err
	}
	defs, err := acquire.DefinitionsFromCore(core.KindMovie, stored)
	if err != nil {
		return nil, err
	}
	delays, err := r.Store.DelayProfiles().List(ctx)
	if err != nil {
		return nil, err
	}
	env := &acquire.Env{Now: r.now, Config: cfg, Definitions: defs}
	for _, d := range delays {
		p, err := acquire.DelayProfileFromCore(d)
		if err != nil {
			return nil, err
		}
		env.DelayProfiles = append(env.DelayProfiles, p)
	}
	rps, err := r.Store.ReleaseProfiles().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range rps {
		env.ReleaseProfiles = append(env.ReleaseProfiles, acquire.ReleaseProfileFromCore(p))
	}
	cfs, err := r.Store.CustomFormats().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range cfs {
		cf, err := acquire.CustomFormatFromCore(f)
		if err != nil {
			return nil, err
		}
		env.Formats = append(env.Formats, cf)
	}
	rn := &run{
		rt: r, settings: s, env: env,
		profiles: map[core.ID]*acquire.QualityProfile{}, libs: map[core.ID]core.Library{}, movies: map[core.ID]*movie{},
		shows: map[core.ID]*show{}, holders: map[core.ID]core.ID{},
	}
	env.Ports = ports{rn}
	rn.engine = &acquire.Engine{Env: env, Specs: acquire.MovieSpecs(), Logger: r.logger()}

	stored, err = r.Store.QualityDefinitions().List(ctx, core.KindSeries)
	if err != nil {
		return nil, err
	}
	sdefs, err := acquire.DefinitionsFromCore(core.KindSeries, stored)
	if err != nil {
		return nil, err
	}
	senv := *env
	senv.Definitions = sdefs
	rn.senv = &series.Env{Env: &senv, Episodes: episodePorts{rn}, AutoUnmonitorPreviouslyDownloaded: s.AutoUnmonitorPreviouslyDownloadedEpisodes}
	rn.sengine = &series.Engine{Env: rn.senv, Specs: series.Specs(), Logger: r.logger()}
	return rn, nil
}

// indexer returns an indexer by its decisions' ID, listing the indexers
// on first use.
func (rn *run) indexer(ctx context.Context, id string) (releases.Indexer, bool) {
	rn.listIndexers(ctx)
	ix, ok := rn.indexers[id]
	return ix, ok
}

// indexerByName returns the first indexer, by provider, priority and
// name, of a name, listing the indexers on first use.
func (rn *run) indexerByName(ctx context.Context, name string) (releases.Indexer, bool) {
	rn.listIndexers(ctx)
	for _, ix := range rn.indexerList {
		if ix.GetName() == name {
			return ix, true
		}
	}
	return releases.Indexer{}, false
}

// listIndexers lists the indexers unless they were.
func (rn *run) listIndexers(ctx context.Context) {
	if rn.indexers != nil {
		return
	}
	rn.indexers = map[string]releases.Indexer{}
	rn.indexerList, _ = rn.rt.searcher().ListIndexers(ctx)
	for _, ix := range rn.indexerList {
		rn.indexers[ix.Provider+"/"+ix.GetId()] = ix
	}
}

// seriesFormats returns the custom formats a title matches, parsed as an
// episode release, of a quality and languages, as Sonarr's
// CustomFormatCalculationService.
func (rn *run) seriesFormats(title string, q acquire.QualityModel, langs []release.Language, size int64, flags []string) []acquire.CustomFormat {
	in := &acquire.Input{Title: title, Quality: release.ParsedQuality{Quality: q.Quality, Revision: q.Revision}, Languages: langs, Size: size, Flags: flags}
	if parsed := release.ParseSeriesTitle(title); parsed != nil {
		in.ReleaseGroup, in.ReleaseType = parsed.ReleaseGroup, parsed.ReleaseType()
	}
	return acquire.MatchFormats(rn.env.Formats, in)
}

// formatsOf returns the custom formats of a title of a movie or episode
// release.
func (rn *run) formatsOf(episodes bool, title string, q acquire.QualityModel, langs []release.Language, size int64, flags []string) []acquire.CustomFormat {
	if episodes {
		return rn.seriesFormats(title, q, langs, size, flags)
	}
	return rn.formats(title, q, langs, size, flags)
}

// formats returns the custom formats a title matches, parsed as a movie
// release, of a quality and languages.
func (rn *run) formats(title string, q acquire.QualityModel, langs []release.Language, size int64, flags []string) []acquire.CustomFormat {
	in := &acquire.Input{Title: title, Quality: release.ParsedQuality{Quality: q.Quality, Revision: q.Revision}, Languages: langs, Size: size, Flags: flags}
	if parsed := release.ParseMovieTitle(title, false); parsed != nil {
		if parsed.SimpleReleaseTitle != "" {
			in.Title = parsed.SimpleReleaseTitle
		}
		in.ReleaseGroup, in.Edition, in.Year = parsed.ReleaseGroup, parsed.Edition, parsed.Year
	}
	return acquire.MatchFormats(rn.env.Formats, in)
}

// qualityModel returns the quality of a stored one.
func qualityModel(q core.QualityRef) acquire.QualityModel {
	rq, _ := acquire.QualityByID(q.Quality)
	return acquire.QualityModel{Quality: rq, Revision: release.Revision{Version: max(q.Version, 1), Real: q.Real, IsRepack: q.IsRepack}}
}

// qualityRef returns the stored form of a quality.
func qualityRef(m acquire.QualityModel) core.QualityRef {
	return core.QualityRef{Quality: acquire.QualityID(m.Quality), Version: m.Revision.Version, Real: m.Revision.Real, IsRepack: m.Revision.IsRepack}
}

func languagesOf(names []string) []release.Language {
	var out []release.Language
	for _, n := range names {
		if l, ok := release.LanguageByName(n); ok {
			out = append(out, l)
		}
	}
	return out
}

func languageNames(langs []release.Language) []string {
	out := make([]string, len(langs))
	for i, l := range langs {
		out[i] = l.String()
	}
	return out
}

// ports are the decisions' view of the store, the plugins and the disks.
type ports struct{ rn *run }

var historyTypes = map[core.HistoryEventType]acquire.HistoryEventType{
	core.HistoryGrabbed:        acquire.HistoryGrabbed,
	core.HistoryImported:       acquire.HistoryImported,
	core.HistoryDownloadFailed: acquire.HistoryDownloadFailed,
	core.HistoryFileDeleted:    acquire.HistoryFileDeleted,
	core.HistoryFileRenamed:    acquire.HistoryFileRenamed,
	core.HistoryIgnored:        acquire.HistoryIgnored,
}

// History implements acquire.Ports. The events of an episode a
// multi-episode file covers beyond its first are those of the item
// holding it.
func (p ports) History(ctx context.Context, itemIDs []core.ID) ([]acquire.HistoryEvent, error) {
	asked := map[core.ID][]core.ID{}
	var query []core.ID
	for _, id := range itemIDs {
		h := p.rn.holderOf(id)
		if _, ok := asked[h]; !ok {
			query = append(query, h)
		}
		asked[h] = append(asked[h], id)
	}
	page, err := p.rn.rt.Store.History().List(ctx, core.HistoryQuery{ItemIDs: query})
	if err != nil {
		return nil, err
	}
	out := make([]acquire.HistoryEvent, 0, len(page.Items))
	for _, h := range page.Items {
		q := qualityModel(h.Quality)
		e := acquire.HistoryEvent{
			Type: historyTypes[h.Type], ItemID: h.ItemID, Date: h.Date, DownloadID: h.DownloadID, SourceTitle: h.SourceTitle,
			Quality: q, Formats: p.rn.formatsOf(!h.ParentID.IsZero(), h.SourceTitle, q, languagesOf(h.Languages), 0, nil),
		}
		ids := asked[h.ItemID]
		if len(ids) == 0 {
			out = append(out, e)
		}
		for _, id := range ids {
			e.ItemID = id
			out = append(out, e)
		}
	}
	return out, nil
}

// Queue implements acquire.Ports.
func (p ports) Queue(ctx context.Context) ([]acquire.QueueEntry, error) {
	tracked, err := p.rn.rt.unfinished(ctx, core.NilID)
	if err != nil {
		return nil, err
	}
	out := make([]acquire.QueueEntry, 0, len(tracked))
	for _, d := range tracked {
		q := qualityModel(d.Quality)
		ids := d.ItemIDs
		if len(ids) == 0 {
			ids = []core.ID{d.ItemID}
		}
		out = append(out, acquire.QueueEntry{
			TargetID: d.ItemID, ItemIDs: ids, Title: d.Title, Size: d.Size, Quality: q,
			Formats:       p.rn.formatsOf(isSeriesDownload(d), d.Title, q, languagesOf(d.Languages), d.Size, nil),
			FailedPending: d.State == core.DownloadFailedPending,
		})
	}
	return out, nil
}

// Pending implements acquire.Ports.
func (p ports) Pending(ctx context.Context, targetID core.ID) ([]acquire.Release, error) {
	pending, err := p.rn.rt.pending(ctx, targetID)
	if err != nil {
		return nil, err
	}
	out := make([]acquire.Release, len(pending))
	for i, pr := range pending {
		out[i] = *pr.release.decisionRelease()
	}
	return out, nil
}

// Blocklisted implements acquire.Ports.
func (p ports) Blocklisted(ctx context.Context, targetID core.ID, r *acquire.Release) (bool, error) {
	var entries []core.BlocklistEntry
	for offset := 0; ; {
		page, err := p.rn.rt.Store.Blocklist().List(ctx, targetID, core.MaxPageSize, offset)
		if err != nil {
			return false, err
		}
		entries = append(entries, page.Items...)
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			break
		}
	}
	return acquire.Blocklisted(entries, r), nil
}

// Indexer implements acquire.Ports: the indexer's settings as its plugin
// tells them.
func (p ports) Indexer(ctx context.Context, id string) (*acquire.IndexerInfo, error) {
	ix, ok := p.rn.indexer(ctx, id)
	if !ok {
		return nil, nil
	}
	return indexerInfo(ix), nil
}

// IndexerByName implements acquire.Ports.
func (p ports) IndexerByName(ctx context.Context, name string) (*acquire.IndexerInfo, error) {
	ix, ok := p.rn.indexerByName(ctx, name)
	if !ok {
		return nil, nil
	}
	return indexerInfo(ix), nil
}

// indexerInfo returns what the decisions need of an indexer. Languages
// of MULTi releases the server does not know are left out.
func indexerInfo(ix releases.Indexer) *acquire.IndexerInfo {
	info := &acquire.IndexerInfo{
		ID: ix.Provider + "/" + ix.GetId(), Name: ix.GetName(), Tags: ix.GetTags(),
		RequiredFlags: ix.GetRequiredFlags(), MinimumSeeders: int(ix.GetMinimumSeeders()),
	}
	for _, name := range ix.GetMultiLanguages() {
		if l, ok := release.LanguageByName(name); ok {
			info.MultiLanguages = append(info.MultiLanguages, l)
		}
	}
	if ix.HasDisabledUntil() {
		info.DisabledUntil = ix.GetDisabledUntil().AsTime()
	}
	return info
}

// FreeSpace implements acquire.Ports.
func (p ports) FreeSpace(_ context.Context, path string) (int64, bool, error) {
	free, ok := freeSpace(path)
	return free, ok, nil
}

// isSeriesDownload reports whether a download is of episodes: a movie's
// download is of the movie alone.
func isSeriesDownload(d core.Download) bool {
	return len(d.ItemIDs) > 1 || (len(d.ItemIDs) == 1 && d.ItemIDs[0] != d.ItemID)
}

// unfinished returns the tracked downloads not yet finished, of an item
// or, for NilID, all.
func (r *Runtime) unfinished(ctx context.Context, itemID core.ID) ([]core.Download, error) {
	var out []core.Download
	for offset := 0; ; {
		page, err := r.Store.Downloads().List(ctx, core.DownloadQuery{ItemID: itemID, Unfinished: true, Limit: core.MaxPageSize, Offset: offset})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return out, nil
		}
	}
}
