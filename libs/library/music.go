package library

import (
	"path"

	"github.com/mavioai/mavio/libs/core"
)

// resolveMusic resolves a folder of a music library: a folder with audio
// files, directly or in disc folders such as "CD1", is an album; a folder
// of albums is an artist.
func (r *Resolver) resolveMusic(scope Scope, dir string, entries []Entry, l Lister) (Result, error) {
	files, dirs := split(entries)
	var res Result
	tracks := r.tracks(files)
	top := dir == scope.Root
	var discs, others []Entry
	for _, d := range dirs {
		if r.Parser.IsMultiPartAlbum(d.Path) {
			discs = append(discs, d)
		} else {
			others = append(others, d)
		}
	}
	descend := func(c Container) {
		for _, d := range others {
			res.Subfolders = append(res.Subfolders, Subfolder{d.Path, Scope{Kind: scope.Kind, Root: scope.Root, Parent: c, ParentPath: dir}})
		}
	}
	if top {
		res.Items = tracks
		descend(InFolder)
		return res, nil
	}
	if len(tracks) > 0 || len(discs) > 0 {
		album := Node{Kind: core.KindMusicAlbum, Path: dir, Name: path.Base(dir)}
		res.Item = &album
		res.Items = tracks
		for i, d := range discs {
			if l == nil {
				break
			}
			sub, err := l.List(d.Path)
			if err != nil {
				return res, err
			}
			subFiles, _ := split(sub)
			disc := i + 1
			for _, t := range r.tracks(subFiles) {
				t.ParentIndex = &disc
				res.Items = append(res.Items, t)
			}
		}
		descend(InFolder)
		return res, nil
	}
	if scope.Parent == InFolder && l != nil {
		for _, d := range others {
			sub, err := l.List(d.Path)
			if err != nil {
				return res, err
			}
			subFiles, subDirs := split(sub)
			hasDiscs := false
			for _, s := range subDirs {
				hasDiscs = hasDiscs || r.Parser.IsMultiPartAlbum(s.Path)
			}
			if len(r.tracks(subFiles)) > 0 || hasDiscs {
				artist := Node{Kind: core.KindMusicArtist, Path: dir, Name: path.Base(dir)}
				res.Item = &artist
				descend(InMusicArtist)
				return res, nil
			}
		}
	}
	descend(InFolder)
	return res, nil
}

// tracks makes a track of every audio file; cue sheets are not tracks.
func (r *Resolver) tracks(files []Entry) []Node {
	var nodes []Node
	for _, f := range files {
		if !r.Parser.IsAudioFile(f.Path) || path.Ext(f.Path) == ".cue" {
			continue
		}
		nodes = append(nodes, Node{Kind: core.KindTrack, Path: f.Path, Name: fileNameWithoutExt(f.Path), InMixedFolder: true})
	}
	return nodes
}
