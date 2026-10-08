package metadata

import "strings"

// DiscType is the kind of disc folder a video was ripped to.
type DiscType string

// Disc folder types.
const (
	DiscNone   DiscType = ""
	DiscDVD    DiscType = "dvd"
	DiscBluRay DiscType = "bluray"
)

// MovieNFOOptions describes the movie whose NFO paths are wanted.
type MovieNFOOptions struct {
	// Disc is set when the movie's path is a DVD or Blu-ray folder.
	Disc DiscType
	// IsStub is set for placeholder files of offline discs.
	IsStub bool
	// InMixedFolder is set when the movie's folder holds other videos, so
	// that a "movie.nfo" would be ambiguous.
	InMixedFolder bool
	// IsMovie is false for other videos, such as extras, which never read
	// "movie.nfo".
	IsMovie bool
}

// MovieNFOPaths returns where the NFO file of a movie or video at path may
// be, most specific first: VIDEO_TS/VIDEO_TS.nfo in a DVD folder,
// movie.nfo in the movie's own folder, and the folder's or file's name with
// an .nfo extension.
func MovieNFOPaths(path string, o MovieNFOOptions) []string {
	sep := "/"
	if strings.Contains(path, `\`) && !strings.Contains(path, "/") {
		sep = `\`
	}
	join := func(parts ...string) string { return strings.Join(parts, sep) }

	folder := path
	if o.Disc == DiscNone {
		if i := strings.LastIndexAny(path, `/\`); i >= 0 {
			folder = path[:i]
		}
	}
	var paths []string
	if o.Disc == DiscDVD && !o.IsStub {
		paths = append(paths, join(folder, "VIDEO_TS", "VIDEO_TS.nfo"))
	}
	if !o.InMixedFolder && o.IsMovie {
		paths = append(paths, join(folder, "movie.nfo"))
	}
	if !o.IsStub && o.Disc != DiscNone {
		paths = append(paths, join(folder, folder[strings.LastIndexAny(folder, `/\`)+1:]+".nfo"))
	} else {
		paths = append(paths, changeExtension(path, ".nfo"))
	}
	return paths
}

// changeExtension replaces the extension of the last path component.
func changeExtension(path, ext string) string {
	name := path[strings.LastIndexAny(path, `/\`)+1:]
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return path[:len(path)-len(name)+i] + ext
	}
	return path + ext
}
