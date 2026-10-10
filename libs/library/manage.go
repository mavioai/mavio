package library

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
	"github.com/mavioai/mavio/libs/naming"
)

// Searcher is a Provider that lists what an item may be.
type Searcher interface {
	Search(ctx context.Context, l Lookup, limit int) ([]SearchResult, error)
}

// SearchResult is something a provider found.
type SearchResult struct {
	// Provider names the provider that found it.
	Provider    string
	Name        string
	Year        int
	Overview    string
	ExternalIDs map[core.Provider]string
	ImageURL    string
	Score       float64
}

// RemoteImage is an image a provider has of an item.
type RemoteImage struct {
	Provider string
	metadata.RemoteImage
}

// SearchQuery overrides what a search is told about an item; zero fields
// are taken from the item.
type SearchQuery struct {
	Name        string
	Year        int
	ExternalIDs map[core.Provider]string
	// Provider restricts the search to one provider.
	Provider string
	// Limit is the number of results per provider; zero means 10.
	Limit int
}

// Search asks the providers that search for what an item of lib may be.
// A provider failing leaves the others' results.
func (r *Refresher) Search(ctx context.Context, lib core.Library, itemID core.ID, q SearchQuery) ([]SearchResult, error) {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return nil, err
	}
	l, err := r.lookup(ctx, lib, it)
	if err != nil {
		return nil, err
	}
	if q.Name != "" || q.Year != 0 || len(q.ExternalIDs) > 0 {
		// A search by name or year must not be steered by the IDs of what
		// the item was taken for.
		l.Name, l.ExternalIDs = cmp.Or(q.Name, l.Name), q.ExternalIDs
		if q.Year != 0 {
			l.Year = q.Year
		}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	var out []SearchResult
	for _, p := range r.providers() {
		s, ok := p.(Searcher)
		if !ok || q.Provider != "" && p.Name() != q.Provider {
			continue
		}
		found, err := s.Search(ctx, l, limit)
		if err != nil {
			r.logger().WarnContext(ctx, "metadata search failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, f := range found {
			f.Provider = p.Name()
			out = append(out, f)
		}
	}
	return out, nil
}

// Identify takes an item of lib for what the external IDs name, such as a
// search result, and refreshes it, replacing its metadata.
func (r *Refresher) Identify(ctx context.Context, lib core.Library, itemID core.ID, ids map[core.Provider]string) (core.Item, error) {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return it, err
	}
	it.ExternalIDs = mergeIDs(nil, ids)
	if len(it.ExternalIDs) == 0 {
		return it, fmt.Errorf("%w: no external IDs to identify the item by", core.ErrInvalid)
	}
	if err := r.Store.Items().Upsert(ctx, it); err != nil {
		return it, err
	}
	return r.RefreshWith(ctx, lib, itemID, RefreshOptions{ReplaceMetadata: true})
}

// RemoteImages lists the images the metadata and image providers have of
// an item of lib, of one kind or, when kind is empty, of all.
func (r *Refresher) RemoteImages(ctx context.Context, lib core.Library, itemID core.ID, kind core.ImageKind) ([]RemoteImage, error) {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return nil, err
	}
	l, err := r.lookup(ctx, lib, it)
	if err != nil {
		return nil, err
	}
	var out []RemoteImage
	for _, p := range r.providers() {
		res, err := p.Metadata(ctx, l)
		if err != nil {
			r.logger().WarnContext(ctx, "metadata provider failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		if res == nil {
			continue
		}
		l.ExternalIDs = mergeIDs(l.ExternalIDs, res.Item.ExternalIDs)
		for _, img := range res.RemoteImages {
			if kind == "" || img.Kind == kind {
				out = append(out, RemoteImage{Provider: p.Name(), RemoteImage: img})
			}
		}
	}
	for _, p := range r.imageProviders() {
		images, err := p.Images(ctx, l)
		if err != nil {
			r.logger().WarnContext(ctx, "image provider failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, img := range images {
			if kind == "" || img.Kind == kind {
				out = append(out, RemoteImage{Provider: p.Name(), RemoteImage: img})
			}
		}
	}
	return out, nil
}

// Update changes an item of lib with change, which may fail, and saves it.
func (r *Refresher) Update(ctx context.Context, lib core.Library, itemID core.ID, change func(*core.Item) error) (core.Item, error) {
	var it core.Item
	err := r.Store.InTx(ctx, func(tx core.Store) error {
		var err error
		if it, err = tx.Items().Get(ctx, itemID); err != nil {
			return err
		}
		if err := change(&it); err != nil {
			return err
		}
		country := cmp.Or(it.MetadataCountry, lib.MetadataCountry)
		it.ParentalRating = ratingScore(&it, country)
		if err := it.Validate(); err != nil {
			return err
		}
		return tx.Items().Upsert(ctx, it)
	})
	if err != nil {
		return it, err
	}
	// The store derives some fields, such as the sort name.
	if it, err = r.Store.Items().Get(ctx, itemID); err != nil {
		return it, err
	}
	return it, r.changed(ctx, lib, it)
}

// changed writes an item's metadata next to its media and puts it into
// its collection, as its library asks.
func (r *Refresher) changed(ctx context.Context, lib core.Library, it core.Item) error {
	var errs []error
	if lib.SaveLocalMetadata {
		errs = append(errs, r.saveLocal(ctx, lib, it))
	}
	if lib.AutoCollections {
		errs = append(errs, r.collect(ctx, it))
	}
	return errors.Join(errs...)
}

// clearMetadata forgets what providers gave an item, except its name,
// year and external IDs, which identify it, and locked fields.
func clearMetadata(it *core.Item) {
	locked := func(f core.MetadataField) bool { return slices.Contains(it.LockedFields, f) }
	if !locked(core.FieldName) {
		it.OriginalTitle, it.SortName = "", ""
	}
	if !locked(core.FieldOverview) {
		it.Overview = ""
	}
	if !locked(core.FieldOfficialRating) {
		it.OfficialRating = ""
	}
	if !locked(core.FieldGenres) {
		it.Genres = nil
	}
	if !locked(core.FieldTags) {
		it.Tags = nil
	}
	if !locked(core.FieldStudios) {
		it.Studios = nil
	}
	if !locked(core.FieldProductionLocations) {
		it.ProductionLocations = nil
	}
	it.Tagline, it.CollectionName, it.AirTime = "", "", ""
	it.PremiereDate, it.EndDate = nil, nil
	it.CommunityRating, it.CriticRating = 0, 0
	it.SeriesStatus, it.AirDays, it.RemoteTrailers = "", nil, nil
}

// saveLocal writes an item's NFO file next to its media and saves its
// provider images there.
func (r *Refresher) saveLocal(ctx context.Context, lib core.Library, it core.Item) error {
	root, rel, ok := libraryRoot(lib, it.Path)
	if !ok || it.Extra != "" {
		return nil
	}
	rt, err := os.OpenRoot(filepath.FromSlash(root))
	if err != nil {
		return fmt.Errorf("save metadata of %s: %w", it.ID, err)
	}
	defer rt.Close()
	images, err := r.Store.Images().ListForOwner(ctx, it.ID)
	if err != nil {
		return err
	}
	// Provider images become files beside the media.
	saved := false
	for i, img := range images {
		if img.Path != "" || img.RemoteURL == "" || r.Fetch == nil {
			continue
		}
		base, ok := artBase(rt.FS(), &it, rel, img.Kind, img.Index)
		if !ok {
			continue
		}
		data, err := r.Fetch(ctx, img.RemoteURL)
		if err != nil {
			r.logger().WarnContext(ctx, "download image failed", "item", it.ID, "url", img.RemoteURL, "err", err)
			continue
		}
		file, err := writeArt(rt, base, data)
		if err != nil {
			return fmt.Errorf("save image of %s: %w", it.ID, err)
		}
		images[i].Path, images[i].RemoteURL = path.Join(root, file), ""
		saved = true
	}
	if saved {
		if err := r.Store.Images().Replace(ctx, it.ID, images); err != nil {
			return err
		}
	}
	if !metadata.CanWriteNFO(it.Kind) {
		return nil
	}
	res := &metadata.Result{Item: it}
	for _, img := range images {
		if img.RemoteURL != "" {
			res.RemoteImages = append(res.RemoteImages, metadata.RemoteImage{Kind: img.Kind, URL: img.RemoteURL})
		}
	}
	if res.People, err = r.people(ctx, it.ID); err != nil {
		return err
	}
	if it.Kind == core.KindEpisode {
		res.SeriesName = r.seriesName(ctx, it)
	}
	data, err := metadata.WriteNFO(res)
	if err != nil {
		return err
	}
	target := nfoTarget(rt.FS(), it, rel, root)
	if old, err := fs.ReadFile(rt.FS(), target); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := writeFileIn(rt, target, data); err != nil {
		return fmt.Errorf("save NFO of %s: %w", it.ID, err)
	}
	return nil
}

// people returns an item's credits as an NFO file records them.
func (r *Refresher) people(ctx context.Context, itemID core.ID) ([]metadata.Person, error) {
	credits, err := r.Store.People().CreditsForItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	out := make([]metadata.Person, 0, len(credits))
	for _, c := range credits {
		p, err := r.Store.People().Get(ctx, c.PersonID)
		if err != nil {
			return nil, err
		}
		order := c.Order
		out = append(out, metadata.Person{Name: p.Name, Kind: c.Kind, Role: c.Role, Order: &order})
	}
	return out, nil
}

// seriesName returns the name of an episode's series, empty when unknown.
func (r *Refresher) seriesName(ctx context.Context, it core.Item) string {
	for id := it.ParentID; !id.IsZero(); {
		p, err := r.Store.Items().Get(ctx, id)
		if err != nil {
			return ""
		}
		if p.Kind == core.KindSeries {
			return p.Name
		}
		id = p.ParentID
	}
	return ""
}

// nfoTarget returns where an item's NFO file is written, relative to its
// library folder: over the one found, else where Jellyfin writes it.
func nfoTarget(fsys fs.FS, it core.Item, rel, root string) string {
	paths := nfoPaths(it, rel, root)
	for _, p := range paths {
		if info, err := fs.Stat(fsys, p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	switch it.Kind {
	case core.KindMovie, core.KindVideo, core.KindMusicVideo:
		if info, err := fs.Stat(fsys, rel); err == nil && !info.IsDir() {
			return strings.TrimSuffix(rel, path.Ext(rel)) + ".nfo"
		}
	}
	return paths[0]
}

// artNames are the names artwork is saved under, as localImages finds it.
var artNames = map[core.ImageKind]string{
	core.ImagePrimary:  "poster",
	core.ImageBackdrop: "fanart",
	core.ImageLogo:     "logo",
	core.ImageArt:      "clearart",
	core.ImageDisc:     "disc",
	core.ImageBanner:   "banner",
	core.ImageThumb:    "landscape",
}

// artBase returns the path, relative to the library folder and without an
// extension, an item's image is saved at beside its media. Images of
// seasons, tracks and photos, and of episodes other than their primary
// image, are not saved there.
func artBase(fsys fs.FS, it *core.Item, rel string, kind core.ImageKind, index int) (string, bool) {
	name, ok := artNames[kind]
	if !ok || index > 0 && kind != core.ImageBackdrop {
		return "", false
	}
	if index > 0 {
		name += "-" + strconv.Itoa(index)
	}
	info, err := fs.Stat(fsys, rel)
	if err != nil {
		return "", false
	}
	switch it.Kind {
	case core.KindSeason, core.KindTrack, core.KindPhoto:
		return "", false
	case core.KindEpisode:
		if kind != core.ImagePrimary || info.IsDir() {
			return "", false
		}
		return strings.TrimSuffix(rel, path.Ext(rel)) + "-thumb", true
	case core.KindMusicAlbum, core.KindMusicArtist, core.KindPhotoAlbum:
		if kind == core.ImagePrimary {
			name = "folder"
		}
	}
	if info.IsDir() {
		return path.Join(rel, name), true
	}
	dir, stem := path.Dir(rel), strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	if mixedFolder(fsys, dir) {
		name = stem + "-" + name
	}
	return path.Join(dir, name), true
}

// mixedFolder reports whether a folder holds several videos, whose
// artwork is then named after them.
func mixedFolder(fsys fs.FS, dir string) bool {
	entries, _ := fs.ReadDir(fsys, dir)
	videos := 0
	for _, e := range entries {
		if !e.IsDir() && naming.Default().IsVideoFile(e.Name()) {
			videos++
		}
	}
	return videos > 1
}

// artFormats are the extensions of the image formats taken, by
// detected content type.
var artFormats = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
}

// imageExtension returns the extension of an image's format.
func imageExtension(data []byte) (string, error) {
	ext, ok := artFormats[http.DetectContentType(data)]
	if !ok {
		return "", fmt.Errorf("%w: not a JPEG, PNG, WebP or GIF image", core.ErrInvalid)
	}
	return ext, nil
}

// writeArt writes an image at base plus its extension within rt, removing
// the artwork of other formats at base, which would be found first, and
// returns its path.
func writeArt(rt *os.Root, base string, data []byte) (string, error) {
	ext, err := imageExtension(data)
	if err != nil {
		return "", err
	}
	if entries, err := fs.ReadDir(rt.FS(), path.Dir(base)); err == nil {
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.EqualFold(strings.TrimSuffix(n, path.Ext(n)), path.Base(base)) &&
				slices.Contains(artworkExtensions, strings.ToLower(path.Ext(n))) {
				if err := rt.Remove(path.Join(path.Dir(base), n)); err != nil {
					return "", err
				}
			}
		}
	}
	file := base + ext
	return file, writeFileIn(rt, file, data)
}

// writeFileIn replaces a file within rt atomically.
func writeFileIn(rt *os.Root, name string, data []byte) error {
	if err := rt.MkdirAll(path.Dir(name), 0o755); err != nil {
		return err
	}
	tmp := path.Join(path.Dir(name), "."+path.Base(name)+".tmp")
	if err := rt.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := rt.Rename(tmp, name); err != nil {
		_ = rt.Remove(tmp)
		return err
	}
	return nil
}

// storedFolder is an item's folder within MetadataDir.
func (r *Refresher) storedFolder(id core.ID) string {
	return path.Join(id.String()[:2], id.String())
}

// storedArt finds the artwork chosen for an item that is kept in
// MetadataDir: "<kind>.<ext>", and "backdrop-<n>.<ext>" for further
// backdrops.
func (r *Refresher) storedArt(it core.Item) []metadata.LocalImage {
	if r.MetadataDir == "" {
		return nil
	}
	dir := filepath.Join(r.MetadataDir, filepath.FromSlash(r.storedFolder(it.ID)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []metadata.LocalImage
	for _, e := range entries {
		n := e.Name()
		ext := path.Ext(n)
		if e.IsDir() || !slices.Contains(artworkExtensions, strings.ToLower(ext)) {
			continue
		}
		kind, _, _ := strings.Cut(strings.TrimSuffix(n, ext), "-")
		if k := core.ImageKind(kind); k.Valid() {
			out = append(out, metadata.LocalImage{Kind: k, Path: filepath.ToSlash(filepath.Join(dir, n))})
		}
	}
	slices.SortStableFunc(out, func(a, b metadata.LocalImage) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// SetImage makes an image the first of its kind of an item of lib. It is
// saved beside the media in libraries saving local metadata, else in
// MetadataDir, where refreshes find it again.
func (r *Refresher) SetImage(ctx context.Context, lib core.Library, itemID core.ID, kind core.ImageKind, data []byte) (core.Image, error) {
	if !kind.Valid() {
		return core.Image{}, fmt.Errorf("%w: unknown image kind %q", core.ErrInvalid, kind)
	}
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return core.Image{}, err
	}
	file, err := r.saveImage(lib, it, kind, data)
	if err != nil {
		return core.Image{}, err
	}
	img := core.Image{ID: core.NewID(), OwnerID: it.ID, Kind: kind, Path: file}
	err = r.Store.InTx(ctx, func(tx core.Store) error {
		images, err := tx.Images().ListForOwner(ctx, it.ID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(images, func(old core.Image) bool { return old.Kind == kind && old.Index == 0 })
		if i >= 0 {
			images[i] = img
		} else {
			images = append([]core.Image{img}, images...)
		}
		numberImages(images)
		return tx.Images().Replace(ctx, it.ID, images)
	})
	return img, err
}

// saveImage writes an item's chosen image and returns its path.
func (r *Refresher) saveImage(lib core.Library, it core.Item, kind core.ImageKind, data []byte) (string, error) {
	if lib.SaveLocalMetadata {
		if root, rel, ok := libraryRoot(lib, it.Path); ok {
			rt, err := os.OpenRoot(filepath.FromSlash(root))
			if err != nil {
				return "", err
			}
			defer rt.Close()
			if base, ok := artBase(rt.FS(), &it, rel, kind, 0); ok {
				file, err := writeArt(rt, base, data)
				if err != nil {
					return "", err
				}
				return path.Join(root, file), nil
			}
		}
	}
	if r.MetadataDir == "" {
		return "", fmt.Errorf("%w: no folder to keep chosen images in", core.ErrInvalid)
	}
	if err := os.MkdirAll(r.MetadataDir, 0o755); err != nil {
		return "", err
	}
	rt, err := os.OpenRoot(r.MetadataDir)
	if err != nil {
		return "", err
	}
	defer rt.Close()
	file, err := writeArt(rt, path.Join(r.storedFolder(it.ID), string(kind)), data)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(r.MetadataDir, filepath.FromSlash(file))), nil
}

// DeleteImage removes an image of an item of lib, with its file when that
// lies in the library's folders or in MetadataDir.
func (r *Refresher) DeleteImage(ctx context.Context, lib core.Library, itemID, imageID core.ID) error {
	var removed core.Image
	err := r.Store.InTx(ctx, func(tx core.Store) error {
		images, err := tx.Images().ListForOwner(ctx, itemID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(images, func(img core.Image) bool { return img.ID == imageID })
		if i < 0 {
			return fmt.Errorf("image %s: %w", imageID, core.ErrNotFound)
		}
		removed = images[i]
		images = slices.Delete(images, i, i+1)
		numberImages(images)
		return tx.Images().Replace(ctx, itemID, images)
	})
	if err != nil || removed.Path == "" {
		return err
	}
	roots := slices.Clone(lib.Paths)
	if r.MetadataDir != "" {
		roots = append(roots, r.MetadataDir)
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, filepath.FromSlash(removed.Path))
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		rt, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		defer rt.Close()
		if err := rt.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	return nil
}

// collect puts a movie into the collection named after its providers'
// collection, creating it when first needed.
func (r *Refresher) collect(ctx context.Context, it core.Item) error {
	if it.Kind != core.KindMovie || it.Extra != "" || it.CollectionName == "" {
		return nil
	}
	return r.Store.InTx(ctx, func(tx core.Store) error {
		lib, err := CuratedLibrary(ctx, tx, core.LibraryCollections)
		if err != nil {
			return err
		}
		id := it.ExternalIDs[core.ProviderTMDBCollection]
		var c core.Item
		for col, err := range tx.Items().Walk(ctx, core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, Kinds: []core.ItemKind{core.KindCollection}}) {
			if err != nil {
				return err
			}
			if id != "" && col.ExternalIDs[core.ProviderTMDBCollection] == id ||
				id == "" && strings.EqualFold(col.Name, it.CollectionName) {
				c = col
				break
			}
		}
		if c.ID.IsZero() {
			c = core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindCollection, Name: it.CollectionName, DateAdded: r.now()}
			if id != "" {
				c.ExternalIDs = map[core.Provider]string{core.ProviderTMDBCollection: id}
			}
			if err := tx.Items().Upsert(ctx, c); err != nil {
				return err
			}
		}
		links, err := tx.Items().Links(ctx, c.ID)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(links, func(l core.Link) bool { return l.ItemID == it.ID }) {
			return nil
		}
		return tx.Items().ReplaceLinks(ctx, c.ID, append(links, core.Link{ID: core.NewID(), ContainerID: c.ID, ItemID: it.ID}))
	})
}

// curatedNames are the names curated libraries are created with.
var curatedNames = map[core.LibraryKind]string{
	core.LibraryCollections: "Collections",
	core.LibraryPlaylists:   "Playlists",
}

// CuratedLibrary returns the library of a curated kind, creating it when
// first needed.
func CuratedLibrary(ctx context.Context, store core.Store, kind core.LibraryKind) (core.Library, error) {
	find := func() (core.Library, bool, error) {
		libs, err := store.Libraries().List(ctx)
		if err != nil {
			return core.Library{}, false, err
		}
		for _, lib := range libs {
			if lib.Kind == kind {
				return lib, true, nil
			}
		}
		return core.Library{}, false, nil
	}
	if lib, ok, err := find(); err != nil || ok {
		return lib, err
	}
	lib := core.Library{Name: curatedNames[kind], Kind: kind}
	err := store.Libraries().Create(ctx, &lib)
	if errors.Is(err, core.ErrConflict) { // created concurrently
		lib, _, err = find()
	}
	return lib, err
}
