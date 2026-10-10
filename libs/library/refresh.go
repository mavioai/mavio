package library

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
	"github.com/mavioai/mavio/libs/naming"
)

// Lookup is what a metadata provider is told about an item.
type Lookup struct {
	Kind        core.ItemKind
	Name        string
	Year        int
	ExternalIDs map[core.Provider]string
	// SeriesExternalIDs, SeasonNumber and EpisodeNumber place seasons and
	// episodes.
	SeriesExternalIDs           map[core.Provider]string
	SeasonNumber, EpisodeNumber *int
	Album                       string
	Artists                     []string
	// Language (ISO 639-1) and Country (ISO 3166-1 alpha-2) are the
	// library's or the item's preferences.
	Language, Country string
}

// Provider fetches metadata from an external source, such as a metadata
// plugin. It returns nil when it knows nothing of the item.
type Provider interface {
	Name() string
	Metadata(ctx context.Context, l Lookup) (*metadata.Result, error)
}

// ImageProvider finds images of items in an external source, such as an
// image plugin.
type ImageProvider interface {
	Name() string
	Images(ctx context.Context, l Lookup) ([]metadata.RemoteImage, error)
}

// Refresher fills in an item's metadata from external providers and local
// NFO files. Local files win over providers, and both over what a scan
// derived from file names; locked items and fields are left alone. It also
// carries out the changes administrators make (manage.go), and writes
// metadata next to the media in libraries saving local metadata.
type Refresher struct {
	Store     core.Store
	Providers []Provider
	// Source, when set, gives the providers instead of Providers, so that
	// providers come and go while the server runs.
	Source func() []Provider
	// Images gives the image providers, which fill in the kinds of images
	// the metadata providers and local files lack; nil means none.
	Images func() []ImageProvider
	// Processors gives the metadata processors, which adjust the merged
	// metadata of items before it is saved; nil means none.
	Processors func() []Processor
	// Local gives the local metadata readers, whose metadata counts after
	// the providers' and before the NFO file's; nil means none.
	Local func() []LocalReader
	// Savers gives the metadata savers, which write metadata beside the
	// media in libraries saving local metadata; nil means none.
	Savers func() []Saver
	// MetadataDir holds the artwork chosen for items whose library does
	// not save local metadata, one folder per item; empty keeps none.
	MetadataDir string
	// Fetch downloads an image; nil leaves provider images where they are
	// instead of saving them next to the media.
	Fetch  func(ctx context.Context, url string) ([]byte, error)
	Logger *slog.Logger
	Now    func() time.Time
}

// RefreshOptions changes how Refresh works.
type RefreshOptions struct {
	// ReplaceMetadata forgets what providers gave before, except external
	// IDs and locked fields, and ignores the item's NFO file, so that
	// nothing stale outlives the refresh. In libraries saving local
	// metadata, the artwork beside the media is ignored too, to be
	// replaced by the providers'.
	ReplaceMetadata bool
}

func (r *Refresher) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (r *Refresher) providers() []Provider {
	if r.Source != nil {
		return r.Source()
	}
	return r.Providers
}

func (r *Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Refresh refreshes one item of lib.
func (r *Refresher) Refresh(ctx context.Context, lib core.Library, itemID core.ID) error {
	_, err := r.RefreshWith(ctx, lib, itemID, RefreshOptions{})
	return err
}

// RefreshWith refreshes one item of lib and returns it.
func (r *Refresher) RefreshWith(ctx context.Context, lib core.Library, itemID core.ID, opts RefreshOptions) (core.Item, error) {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return it, err
	}
	if it.Locked {
		return it, nil
	}
	var local *metadata.Result
	if !opts.ReplaceMetadata {
		if local, err = r.readNFO(lib, it); err != nil {
			r.logger().WarnContext(ctx, "reading NFO failed", "item", it.ID, "err", err)
		}
	}
	// Chosen artwork comes first, then artwork beside the media, then what
	// the NFO names.
	art := r.storedArt(it)
	if !opts.ReplaceMetadata || !lib.SaveLocalMetadata {
		for _, img := range r.localArt(lib, it) {
			if !slices.ContainsFunc(art, func(a metadata.LocalImage) bool { return a.Kind == img.Kind && img.Kind != core.ImageBackdrop }) {
				art = append(art, img)
			}
		}
	}
	if len(art) > 0 {
		if local == nil {
			local = &metadata.Result{}
		}
		for _, img := range local.LocalImages {
			if !slices.ContainsFunc(art, func(a metadata.LocalImage) bool { return a.Kind == img.Kind }) {
				art = append(art, img)
			}
		}
		local.LocalImages = art
	}
	// Locks recorded in the local file hold against providers too.
	if local != nil {
		if len(local.Item.LockedFields) > 0 {
			it.LockedFields = slices.Clone(local.Item.LockedFields)
		}
		it.Locked = it.Locked || local.Item.Locked
	}
	if opts.ReplaceMetadata {
		clearMetadata(&it)
	}
	lookup, err := r.lookup(ctx, lib, it)
	if err != nil {
		return it, err
	}
	var results []*metadata.Result
	if it.Extra == "" && !it.Locked {
		for _, p := range r.providers() {
			res, err := p.Metadata(ctx, lookup)
			if err != nil {
				// One provider failing leaves the others' metadata.
				r.logger().WarnContext(ctx, "metadata provider failed", "provider", p.Name(), "item", it.ID, "err", err)
				continue
			}
			if res != nil {
				results = append(results, res)
				// Later providers see what earlier ones found.
				lookup.ExternalIDs = mergeIDs(lookup.ExternalIDs, res.Item.ExternalIDs)
			}
		}
	}
	if !opts.ReplaceMetadata && it.Extra == "" && !it.Locked {
		results = append(results, r.readLocal(ctx, lib, it, lookup)...)
	}
	if local != nil {
		results = append(results, local)
	}
	var extraImages []metadata.RemoteImage
	if it.Extra == "" && !it.Locked {
		extraImages = r.providerImages(ctx, lookup)
	}
	name := it.Name
	for _, res := range results {
		applyMetadata(&it, res.Item, res == local)
	}
	if it.Name == "" {
		it.Name = name
	}
	var processedPeople []metadata.Person
	if it.Extra == "" && !it.Locked {
		processedPeople = r.process(ctx, &it, results)
	}
	it.ParentalRating = ratingScore(&it, lookup.Country)
	it.MetadataRefreshedAt = r.now()
	err = r.Store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, it); err != nil {
			return err
		}
		// The most trusted source with people and images provides them.
		credited := slices.Contains(it.LockedFields, core.FieldCast)
		if !credited && processedPeople != nil {
			if err := replaceCredits(ctx, tx, it.ID, processedPeople); err != nil {
				return err
			}
			credited = true
		}
		for i := len(results) - 1; i >= 0 && !credited; i-- {
			if len(results[i].People) > 0 {
				if err := replaceCredits(ctx, tx, it.ID, results[i].People); err != nil {
					return err
				}
				credited = true
			}
		}
		if opts.ReplaceMetadata && !credited {
			if err := tx.People().ReplaceCredits(ctx, it.ID, nil); err != nil {
				return err
			}
		}
		// Each kind of image comes from the most trusted source that has
		// it, then from the image providers.
		var images []core.Image
		for i := len(results) - 1; i >= 0; i-- {
			for _, img := range imagesOf(it.ID, results[i]) {
				if !slices.ContainsFunc(images, func(have core.Image) bool { return have.Kind == img.Kind }) ||
					(img.Kind == core.ImageBackdrop && i == len(results)-1) {
					images = append(images, img)
				}
			}
		}
		for _, img := range imagesOf(it.ID, &metadata.Result{RemoteImages: extraImages}) {
			if !slices.ContainsFunc(images, func(have core.Image) bool { return have.Kind == img.Kind }) {
				images = append(images, img)
			}
		}
		if len(images) == 0 {
			return nil
		}
		numberImages(images)
		existing, err := tx.Images().ListForOwner(ctx, it.ID)
		if err != nil {
			return err
		}
		keepImages(images, existing)
		return tx.Images().Replace(ctx, it.ID, images)
	})
	if err != nil {
		return it, err
	}
	return it, r.changed(ctx, lib, it)
}

// Processor adjusts the merged metadata of items at the end of refreshes,
// such as a metadata processor plugin.
type Processor interface {
	Name() string
	// Process returns metadata to apply over an item's, or nil to leave it
	// as it is. Its people, when given, replace the item's credits.
	Process(ctx context.Context, res *metadata.Result) (*metadata.Result, error)
}

// process has the processors adjust an item's merged metadata, in order,
// each seeing what the ones before did; locked fields are kept. It
// returns the people the last processor that gave any gave.
func (r *Refresher) process(ctx context.Context, it *core.Item, results []*metadata.Result) []metadata.Person {
	if r.Processors == nil {
		return nil
	}
	merged := &metadata.Result{}
	for i := len(results) - 1; i >= 0; i-- {
		if len(results[i].People) > 0 {
			merged.People = results[i].People
			break
		}
	}
	var people []metadata.Person
	for _, p := range r.Processors() {
		merged.Item = *it
		res, err := p.Process(ctx, merged)
		if err != nil {
			r.logger().WarnContext(ctx, "metadata processor failed", "processor", p.Name(), "item", it.ID, "err", err)
			continue
		}
		if res == nil {
			continue
		}
		res.Item.LockedFields, res.Item.Locked = nil, false
		applyMetadata(it, res.Item, false)
		if len(res.People) > 0 {
			merged.People, people = res.People, res.People
		}
	}
	return people
}

// imageProviders returns the image providers.
func (r *Refresher) imageProviders() []ImageProvider {
	if r.Images == nil {
		return nil
	}
	return r.Images()
}

// providerImages asks the image providers for the best image of each kind
// they have of an item, in provider order.
func (r *Refresher) providerImages(ctx context.Context, l Lookup) []metadata.RemoteImage {
	var out []metadata.RemoteImage
	for _, p := range r.imageProviders() {
		images, err := p.Images(ctx, l)
		if err != nil {
			r.logger().WarnContext(ctx, "image provider failed", "provider", p.Name(), "err", err)
			continue
		}
		out = append(out, bestOfEachKind(images)...)
	}
	return out
}

// bestOfEachKind returns the image of each kind with the highest score,
// the first on ties, in the order the kinds come.
func bestOfEachKind(images []metadata.RemoteImage) []metadata.RemoteImage {
	var out []metadata.RemoteImage
	for _, img := range images {
		i := slices.IndexFunc(out, func(o metadata.RemoteImage) bool { return o.Kind == img.Kind })
		switch {
		case i < 0:
			out = append(out, img)
		case img.Score > out[i].Score:
			out[i] = img
		}
	}
	return out
}

// lookup describes an item to providers, with its series and season for
// episodes and seasons.
func (r *Refresher) lookup(ctx context.Context, lib core.Library, it core.Item) (Lookup, error) {
	l := Lookup{
		Kind: it.Kind, Name: it.Name, Year: it.ProductionYear, ExternalIDs: it.ExternalIDs,
		Album: it.Album, Artists: it.Artists, Language: lib.PreferredLanguage, Country: lib.MetadataCountry,
	}
	if it.MetadataLanguage != "" {
		l.Language = it.MetadataLanguage
	}
	if it.MetadataCountry != "" {
		l.Country = it.MetadataCountry
	}
	switch it.Kind {
	case core.KindSeason:
		l.SeasonNumber = it.IndexNumber
	case core.KindEpisode:
		l.SeasonNumber, l.EpisodeNumber = it.ParentIndexNumber, it.IndexNumber
	default:
		return l, nil
	}
	// Seasons and episodes are found through their series.
	for id := it.ParentID; !id.IsZero(); {
		p, err := r.Store.Items().Get(ctx, id)
		if errors.Is(err, core.ErrNotFound) {
			break
		}
		if err != nil {
			return l, err
		}
		if p.Kind == core.KindSeries {
			l.SeriesExternalIDs = p.ExternalIDs
			break
		}
		id = p.ParentID
	}
	return l, nil
}

// defaultRatingCountry is the rating system tried first for items and
// libraries without a metadata country, as in Jellyfin.
const defaultRatingCountry = "US"

// ratingScore returns the score of an item's rating, its custom rating
// before its official one, in the rating system of country; nil when
// unrated or unknown.
func ratingScore(it *core.Item, country string) *int {
	score, ok := metadata.RatingScore(cmp.Or(it.CustomRating, it.OfficialRating), cmp.Or(country, defaultRatingCountry))
	if !ok {
		return nil
	}
	return &score
}

// applyMetadata copies what src knows over dst, except locked fields
// unless src is the local NFO file, which locks do not hold against.
func applyMetadata(dst *core.Item, src core.Item, local bool) {
	locked := func(f core.MetadataField) bool { return !local && slices.Contains(dst.LockedFields, f) }
	str := func(d *string, s string, f core.MetadataField) {
		if s != "" && (f == "" || !locked(f)) {
			*d = s
		}
	}
	list := func(d *[]string, s []string, f core.MetadataField) {
		if len(s) > 0 && !locked(f) {
			*d = slices.Clone(s)
		}
	}
	str(&dst.Name, src.Name, core.FieldName)
	str(&dst.OriginalTitle, src.OriginalTitle, core.FieldName)
	str(&dst.SortName, src.SortName, core.FieldName)
	str(&dst.Overview, src.Overview, core.FieldOverview)
	str(&dst.Tagline, src.Tagline, "")
	str(&dst.OfficialRating, src.OfficialRating, core.FieldOfficialRating)
	str(&dst.CustomRating, src.CustomRating, core.FieldOfficialRating)
	str(&dst.Album, src.Album, "")
	str(&dst.CollectionName, src.CollectionName, "")
	str(&dst.AirTime, src.AirTime, "")
	str(&dst.DisplayOrder, src.DisplayOrder, "")
	list(&dst.Genres, src.Genres, core.FieldGenres)
	list(&dst.Tags, src.Tags, core.FieldTags)
	list(&dst.Studios, src.Studios, core.FieldStudios)
	list(&dst.ProductionLocations, src.ProductionLocations, core.FieldProductionLocations)
	list(&dst.Artists, src.Artists, "")
	list(&dst.AlbumArtists, src.AlbumArtists, "")
	list(&dst.RemoteTrailers, src.RemoteTrailers, "")
	if src.ProductionYear != 0 {
		dst.ProductionYear = src.ProductionYear
	}
	if src.PremiereDate != nil {
		dst.PremiereDate = src.PremiereDate
	}
	if src.EndDate != nil {
		dst.EndDate = src.EndDate
	}
	if src.Runtime != 0 && !locked(core.FieldRuntime) {
		dst.Runtime = src.Runtime
	}
	if src.CommunityRating != 0 {
		dst.CommunityRating = src.CommunityRating
	}
	if src.CriticRating != 0 {
		dst.CriticRating = src.CriticRating
	}
	if src.SeriesStatus != "" {
		dst.SeriesStatus = src.SeriesStatus
	}
	if len(src.AirDays) > 0 {
		dst.AirDays = slices.Clone(src.AirDays)
	}
	// Numbers from files only fill gaps: the folder structure defines them.
	if dst.IndexNumber == nil {
		dst.IndexNumber = src.IndexNumber
	}
	if dst.ParentIndexNumber == nil {
		dst.ParentIndexNumber = src.ParentIndexNumber
	}
	dst.ExternalIDs = mergeIDs(dst.ExternalIDs, src.ExternalIDs)
	if len(src.LockedFields) > 0 {
		dst.LockedFields = slices.Clone(src.LockedFields)
	}
	if src.Locked {
		dst.Locked = true
	}
}

func mergeIDs(dst, src map[core.Provider]string) map[core.Provider]string {
	for k, v := range src {
		if v == "" {
			continue
		}
		if dst == nil {
			dst = map[core.Provider]string{}
		}
		dst[k] = v
	}
	return dst
}

func replaceCredits(ctx context.Context, tx core.Store, itemID core.ID, people []metadata.Person) error {
	credits := make([]core.Credit, 0, len(people))
	for i, p := range people {
		person, err := tx.People().FindByName(ctx, p.Name)
		if errors.Is(err, core.ErrNotFound) {
			person = core.Person{ID: core.NewID(), Name: p.Name}
			if err := tx.People().Upsert(ctx, person); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		order := i
		if p.Order != nil {
			order = *p.Order
		}
		credits = append(credits, core.Credit{PersonID: person.ID, Kind: p.Kind, Role: p.Role, Order: order})
	}
	return tx.People().ReplaceCredits(ctx, itemID, credits)
}

func imagesOf(owner core.ID, res *metadata.Result) []core.Image {
	var images []core.Image
	for _, img := range res.LocalImages {
		images = append(images, core.Image{OwnerID: owner, Kind: img.Kind, Path: img.Path})
	}
	for _, img := range res.RemoteImages {
		if !slices.ContainsFunc(images, func(i core.Image) bool { return i.Kind == img.Kind }) {
			images = append(images, core.Image{OwnerID: owner, Kind: img.Kind, RemoteURL: img.URL})
		}
	}
	return images
}

// numberImages numbers the images of each kind in order.
func numberImages(images []core.Image) {
	next := map[core.ImageKind]int{}
	for i := range images {
		images[i].Index = next[images[i].Kind]
		next[images[i].Kind]++
	}
}

// localArt finds the artwork beside an item in its library folder.
func (r *Refresher) localArt(lib core.Library, it core.Item) []metadata.LocalImage {
	root, rel, ok := libraryRoot(lib, it.Path)
	if !ok {
		return nil
	}
	rt, err := os.OpenRoot(filepath.FromSlash(root))
	if err != nil {
		return nil
	}
	defer rt.Close()
	info, err := fs.Stat(rt.FS(), rel)
	if err != nil {
		return nil
	}
	mixed := false
	if !info.IsDir() {
		// A file shares its folder when other videos lie beside it.
		entries, _ := fs.ReadDir(rt.FS(), path.Dir(rel))
		videos := 0
		for _, e := range entries {
			if !e.IsDir() && naming.Default().IsVideoFile(e.Name()) {
				videos++
			}
		}
		mixed = videos > 1
	}
	images := localImages(rt.FS(), &it, rel, info.IsDir(), mixed)
	for i := range images {
		images[i].Path = path.Join(root, images[i].Path)
	}
	return images
}

// keepImages carries the ID, size and placeholders of images found again
// over to their new records, so that refreshes keep them.
func keepImages(images, existing []core.Image) {
	for i := range images {
		img := &images[i]
		for _, old := range existing {
			if old.Kind == img.Kind && old.Index == img.Index && old.Path == img.Path && old.RemoteURL == img.RemoteURL {
				img.ID, img.Width, img.Height, img.Blurhash, img.Thumbhash = old.ID, old.Width, old.Height, old.Blurhash, old.Thumbhash
				break
			}
		}
	}
}

// readNFO reads the item's local NFO file, through the library folder that
// holds it; nil when there is none.
func (r *Refresher) readNFO(lib core.Library, it core.Item) (*metadata.Result, error) {
	root, rel, ok := libraryRoot(lib, it.Path)
	if !ok {
		return nil, nil
	}
	rt, err := os.OpenRoot(filepath.FromSlash(root))
	if err != nil {
		return nil, err
	}
	defer rt.Close()
	for _, p := range nfoPaths(it, rel, root) {
		data, err := fs.ReadFile(rt.FS(), p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		res, err := metadata.ParseNFO(data, it.Kind, metadata.NFOOptions{
			// Local images count when they lie in the library folder.
			FileExists: func(loc string) bool {
				rel, ok := strings.CutPrefix(filepath.ToSlash(loc), root+"/")
				if !ok {
					return false
				}
				info, err := fs.Stat(rt.FS(), rel)
				return err == nil && info.Mode().IsRegular()
			},
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		// Local images are relative to the NFO file.
		for i, img := range res.LocalImages {
			if !path.IsAbs(img.Path) {
				res.LocalImages[i].Path = path.Join(root, path.Dir(p), img.Path)
			}
		}
		return res, nil
	}
	return nil, nil
}

// libraryRoot returns the library folder holding p, and p relative to it.
func libraryRoot(lib core.Library, p string) (string, string, bool) {
	for _, root := range lib.Paths {
		root = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(root)), "/")
		if rel, ok := strings.CutPrefix(p, root+"/"); ok {
			return root, rel, true
		}
	}
	return "", "", false
}

// nfoPaths lists where an item's NFO file may be, relative to its library
// folder, most specific first.
func nfoPaths(it core.Item, rel, root string) []string {
	dir := path.Dir(rel)
	switch it.Kind {
	case core.KindSeries:
		return []string{path.Join(rel, "tvshow.nfo")}
	case core.KindSeason:
		return []string{path.Join(rel, "season.nfo")}
	case core.KindMusicAlbum:
		return []string{path.Join(rel, "album.nfo")}
	case core.KindMusicArtist:
		return []string{path.Join(rel, "artist.nfo")}
	case core.KindMovie, core.KindVideo, core.KindMusicVideo:
		var paths []string
		for _, p := range metadata.MovieNFOPaths(rel, metadata.MovieNFOOptions{
			// A movie at the top of a library shares its folder.
			InMixedFolder: dir == ".",
			IsMovie:       it.Kind == core.KindMovie && it.Extra == "",
		}) {
			paths = append(paths, path.Clean(p))
		}
		return paths
	}
	return []string{strings.TrimSuffix(rel, path.Ext(rel)) + ".nfo"}
}
