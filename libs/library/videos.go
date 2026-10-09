package library

import (
	"path"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
)

// videos groups the video files of folder dir into items of kind: stacked
// parts become one item, alternate versions attach to the primary one, and
// extras and samples are left out. top marks the library root, whose items
// are always in a mixed folder.
func (r *Resolver) videos(dir string, files []Entry, kind core.ItemKind, parseName, top bool) []Node {
	var vfs []naming.VideoFile
	for _, f := range files {
		if sampleName.MatchString(f.Name()) {
			continue
		}
		if v, ok := r.Parser.ResolveVideo(f.Path, false, parseName, dir); ok {
			vfs = append(vfs, v)
		}
	}
	resolved := r.Parser.ResolveVideos(vfs, naming.VideoListOptions{
		NoNameParsing: !parseName,
		LibraryRoot:   dir,
		TVShows:       kind == core.KindEpisode,
	})
	mixed := len(resolved) > 1 || top
	var nodes []Node
	for _, v := range resolved {
		if v.Extra != "" || len(v.Files) == 0 {
			continue
		}
		first := v.Files[0]
		n := Node{
			Kind:          kind,
			Path:          first.Path,
			Name:          first.Name,
			Year:          v.Year,
			Format3D:      format3D(first.Is3D, first.Format3D),
			InMixedFolder: mixed,
		}
		if parseName {
			n.Name = v.Name
		}
		if first.StubType != "" {
			n.Disc = stubDisc(first.StubType)
		}
		for _, f := range v.Files[1:] {
			n.Parts = append(n.Parts, f.Path)
		}
		for _, alt := range v.AlternateVersions {
			if len(alt.Files) == 0 {
				continue
			}
			ver := Version{Path: alt.Files[0].Path}
			for _, f := range alt.Files[1:] {
				ver.Parts = append(ver.Parts, f.Path)
			}
			n.Versions = append(n.Versions, ver)
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func stubDisc(stubType string) string {
	switch strings.ToLower(stubType) {
	case "dvd":
		return "dvd"
	case "bluray":
		return "bluray"
	}
	return ""
}

// setMovieIDs ports MovieResolver.SetProviderIdsFromPath: TMDB and TVDB
// IDs come from the item's own name, the folder name for movies in their
// own folder with the file name as fallback; IMDb IDs from anywhere in the
// path.
func setMovieIDs(n *Node, ownFolder string) {
	name, fallback := path.Base(n.Path), ""
	if ownFolder != "" {
		name, fallback = path.Base(ownFolder), path.Base(n.Path)
	}
	for attr, provider := range map[string]core.Provider{"tmdbid": core.ProviderTMDB, "tvdbid": core.ProviderTVDB} {
		id, _ := AttributeValue(name, attr)
		if id == "" && fallback != "" {
			id, _ = AttributeValue(fallback, attr)
		}
		if id != "" {
			if n.ExternalIDs == nil {
				n.ExternalIDs = map[core.Provider]string{}
			}
			n.ExternalIDs[provider] = id
		}
	}
	if id, _ := AttributeValue(n.Path, "imdbid"); id != "" {
		if n.ExternalIDs == nil {
			n.ExternalIDs = map[core.Provider]string{}
		}
		n.ExternalIDs[core.ProviderIMDb] = id
	}
}
