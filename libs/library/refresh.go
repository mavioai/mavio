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

// Refresher fills in an item's metadata from external providers and local
// NFO files. Local files win over providers, and both over what a scan
// derived from file names; locked items and fields are left alone.
type Refresher struct {
	Store     core.Store
	Providers []Provider
	Logger    *slog.Logger
	Now       func() time.Time
}

func (r *Refresher) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (r *Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Refresh refreshes one item of lib.
func (r *Refresher) Refresh(ctx context.Context, lib core.Library, itemID core.ID) error {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return err
	}
	if it.Locked {
		return nil
	}
	local, err := r.readNFO(lib, it)
	if err != nil {
		r.logger().WarnContext(ctx, "reading NFO failed", "item", it.ID, "err", err)
	}
	// Artwork beside the media comes first, then what the NFO names.
	if art := r.localArt(lib, it); len(art) > 0 {
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
	lookup, err := r.lookup(ctx, lib, it)
	if err != nil {
		return err
	}
	var results []*metadata.Result
	if it.Extra == "" && !it.Locked {
		for _, p := range r.Providers {
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
	if local != nil {
		results = append(results, local)
	}
	for _, res := range results {
		applyMetadata(&it, res.Item)
	}
	it.ParentalRating = ratingScore(&it, lookup.Country)
	it.MetadataRefreshedAt = r.now()
	return r.Store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, it); err != nil {
			return err
		}
		// The most trusted source with people and images provides them.
		for i := len(results) - 1; i >= 0; i-- {
			if len(results[i].People) > 0 && !slices.Contains(it.LockedFields, core.FieldCast) {
				if err := replaceCredits(ctx, tx, it.ID, results[i].People); err != nil {
					return err
				}
				break
			}
		}
		// Each kind of image comes from the most trusted source that has
		// it.
		var images []core.Image
		for i := len(results) - 1; i >= 0; i-- {
			for _, img := range imagesOf(it.ID, results[i]) {
				if !slices.ContainsFunc(images, func(have core.Image) bool { return have.Kind == img.Kind }) ||
					(img.Kind == core.ImageBackdrop && i == len(results)-1) {
					images = append(images, img)
				}
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

// applyMetadata copies what src knows over dst, except locked fields.
func applyMetadata(dst *core.Item, src core.Item) {
	locked := func(f core.MetadataField) bool { return slices.Contains(dst.LockedFields, f) }
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
