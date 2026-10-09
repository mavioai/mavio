package library

import (
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// Artwork beside media files, as Jellyfin's local image providers find
// it.

// artworkExtensions are the artwork file extensions, preferred first.
var artworkExtensions = []string{".png", ".jpg", ".jpeg", ".webp", ".tbn", ".gif", ".svg"}

// Primary image names by kind of item, preferred first.
var (
	commonImageNames = []string{"poster", "folder", "cover", "default"}
	musicImageNames  = []string{"folder", "poster", "cover", "jacket", "default", "albumart"}
	seriesImageNames = []string{"poster", "folder", "cover", "default", "show"}
	videoImageNames  = []string{"poster", "folder", "cover", "default", "movie"}
)

// artFolder lists a folder's artwork files, preferred extensions first,
// and its subfolders, within a library root.
type artFolder struct {
	fsys  fs.FS
	dir   string // relative to the root
	files []string
	dirs  []string
}

func readArtFolder(fsys fs.FS, dir string) artFolder {
	f := artFolder{fsys: fsys, dir: dir}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return f
	}
	for _, e := range entries {
		switch {
		case e.IsDir():
			f.dirs = append(f.dirs, e.Name())
		case slices.Contains(artworkExtensions, strings.ToLower(path.Ext(e.Name()))):
			f.files = append(f.files, e.Name())
		}
	}
	slices.SortStableFunc(f.files, func(a, b string) int {
		return slices.Index(artworkExtensions, strings.ToLower(path.Ext(a))) - slices.Index(artworkExtensions, strings.ToLower(path.Ext(b)))
	})
	return f
}

// find returns the first non-empty artwork file named prefix+name, without
// regard to case or extension.
func (f artFolder) find(prefix, name string) (string, bool) {
	want := prefix + name
	for _, file := range f.files {
		if !strings.EqualFold(strings.TrimSuffix(file, path.Ext(file)), want) {
			continue
		}
		p := path.Join(f.dir, file)
		if info, err := fs.Stat(f.fsys, p); err == nil && info.Size() > 0 {
			return p, true
		}
	}
	return "", false
}

// localArt collects the images found for an item, keeping one image per
// kind except backdrops.
type localArt struct {
	images []metadata.LocalImage
}

func (a *localArt) add(kind core.ImageKind, p string) {
	for _, img := range a.images {
		if img.Path == p || (img.Kind == kind && kind != core.ImageBackdrop) {
			return
		}
	}
	a.images = append(a.images, metadata.LocalImage{Kind: kind, Path: p})
}

// addFrom adds folder's image prefix+name; added reports whether it exists.
func (a *localArt) addFrom(f artFolder, kind core.ImageKind, prefix, name string) bool {
	p, ok := f.find(prefix, name)
	if ok {
		a.add(kind, p)
	}
	return ok
}

// addNamed adds the image named with the item's prefix and, outside mixed
// folders, without it.
func (a *localArt) addNamed(f artFolder, kind core.ImageKind, prefix, name string, mixed bool) bool {
	added := a.addFrom(f, kind, prefix, name)
	if !mixed && a.addFrom(f, kind, "", name) {
		added = true
	}
	return added
}

// localImages finds the artwork of an item in its library root: rel is
// the item's path relative to the root, isFolder whether the item is its
// folder, and mixed whether the folder holds other items. Episodes take
// "<file>" and "<file>-thumb" images; tracks and photos take none. Paths
// are relative to the root.
func localImages(fsys fs.FS, it *core.Item, rel string, isFolder, mixed bool) []metadata.LocalImage {
	var a localArt
	switch it.Kind {
	case core.KindTrack, core.KindPhoto:
		return nil
	case core.KindEpisode:
		dir, base := path.Dir(rel), strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		for _, d := range []string{dir, path.Join(dir, "metadata")} {
			f := readArtFolder(fsys, d)
			a.addFrom(f, core.ImagePrimary, "", base)
			a.addFrom(f, core.ImagePrimary, base, "-thumb")
		}
		return a.images
	}

	dir, name := path.Dir(rel), strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	if isFolder {
		dir, name = rel, path.Base(rel)
	}
	f := readArtFolder(fsys, dir)
	if it.Kind == core.KindSeason && it.IndexNumber != nil {
		// A season's images may sit in its series' folder.
		series := readArtFolder(fsys, path.Dir(dir))
		prefixes := []string{strings.ToLower(strings.ReplaceAll(it.Name, " ", ""))}
		marker := "-specials"
		if *it.IndexNumber != 0 {
			marker = strconv.Itoa(*it.IndexNumber)
			if len(marker) < 2 {
				marker = "0" + marker
			}
		}
		if !strings.EqualFold(prefixes[0], marker) {
			prefixes = append(prefixes, "season"+marker)
		}
		for _, p := range prefixes {
			a.addFrom(series, core.ImagePrimary, p, "-poster")
			a.addFrom(series, core.ImageBackdrop, p, "-fanart")
			a.addFrom(series, core.ImageBanner, p, "-banner")
			a.addFrom(series, core.ImageThumb, p, "-landscape")
		}
	}

	prefix := name + "-"
	// Primary: the item's own name, then the conventional names.
	names := commonImageNames
	switch it.Kind {
	case core.KindMusicAlbum, core.KindMusicArtist, core.KindPhotoAlbum:
		names = musicImageNames
	case core.KindSeries:
		names = seriesImageNames
	case core.KindMovie, core.KindVideo, core.KindMusicVideo:
		names = videoImageNames
	}
	primary := func(prefix string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return a.addFrom(f, core.ImagePrimary, prefix, n) })
	}
	_ = a.addFrom(f, core.ImagePrimary, "", name) || primary(prefix) || (!mixed && primary(""))

	if !a.addNamed(f, core.ImageLogo, prefix, "logo", mixed) {
		a.addNamed(f, core.ImageLogo, prefix, "clearlogo", mixed)
	}
	a.addNamed(f, core.ImageArt, prefix, "clearart", mixed)
	switch it.Kind {
	case core.KindMusicAlbum:
		if !a.addNamed(f, core.ImageDisc, prefix, "cdart", mixed) {
			a.addNamed(f, core.ImageDisc, prefix, "disc", mixed)
		}
	case core.KindMovie, core.KindVideo, core.KindMusicVideo, core.KindCollection:
		if !a.addNamed(f, core.ImageDisc, prefix, "disc", mixed) && !a.addNamed(f, core.ImageDisc, prefix, "cdart", mixed) {
			a.addNamed(f, core.ImageDisc, prefix, "discart", mixed)
		}
	}
	a.addNamed(f, core.ImageBanner, prefix, "banner", mixed)
	if !a.addNamed(f, core.ImageThumb, prefix, "landscape", mixed) {
		a.addNamed(f, core.ImageThumb, prefix, "thumb", mixed)
	}

	// Backdrops.
	a.addFrom(f, core.ImageBackdrop, prefix, name+"-fanart")
	if !mixed {
		a.addFrom(f, core.ImageBackdrop, "", name+"-fanart")
	}
	numbered := func(first, next string) {
		series := func(p string) {
			a.addFrom(f, core.ImageBackdrop, p, first)
			missed := 0
			for i := 1; i <= 20 && missed < 3; i++ {
				if !a.addFrom(f, core.ImageBackdrop, p, next+strconv.Itoa(i)) {
					missed++
				}
			}
		}
		series(prefix)
		if !mixed {
			series("")
		}
	}
	numbered("fanart", "fanart-")
	numbered("background", "background-")
	numbered("art", "art-")
	if i := slices.IndexFunc(f.dirs, func(d string) bool { return strings.EqualFold(d, "extrafanart") }); i >= 0 {
		extra := readArtFolder(fsys, path.Join(dir, f.dirs[i]))
		for _, file := range extra.files {
			p := path.Join(extra.dir, file)
			if info, err := fs.Stat(fsys, p); err == nil && info.Size() > 0 {
				a.add(core.ImageBackdrop, p)
			}
		}
	}
	numbered("backdrop", "backdrop")
	return a.images
}
