package library

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/subtitle"
)

// languages recognizes languages in file names; the table is built once
// and never changes.
var languages = sync.OnceValue(newLanguageFinder)

// sidecarCodecs are the codecs of subtitle files by extension, named as
// ffprobe names them; other subtitle files need probing and are left out.
var sidecarCodecs = map[string]string{
	".srt": "subrip", ".ass": "ass", ".ssa": "ssa", ".vtt": "webvtt", ".smi": "sami", ".sami": "sami",
	".sup": "hdmv_pgs_subtitle", ".idx": "dvd_subtitle", ".sub": "microdvd",
}

// minSidecarScore is the stem match a subtitle file needs to belong to a
// video: its name is the video's, or starts with it and a delimiter
// (subtitle.ScoreSubtitle).
const minSidecarScore = 500

// sidecars returns the subtitle files beside a video as external streams,
// best first: the subtitle scoring of docs/architecture.md §9.1 without a
// user's languages, so files named exactly like the video, unflagged and
// in richer formats come first. Languages come from the file names,
// Chinese variants included. entries are the folder's files.
func sidecars(parser *naming.Parser, video string, entries map[string]core.FolderEntry) []core.MediaStream {
	dir, stem := path.Dir(video), fileNameWithoutExt(video)
	type candidate struct {
		stream core.MediaStream
		score  int
	}
	var found []candidate
	for p, e := range entries {
		if e.IsDir || path.Dir(p) != dir || p == video {
			continue
		}
		ext := strings.ToLower(path.Ext(p))
		codec, ok := sidecarCodecs[ext]
		if !ok {
			continue
		}
		// A VobSub's .sub goes with its .idx.
		if ext == ".sub" {
			if _, ok := entries[strings.TrimSuffix(p, path.Ext(p))+".idx"]; ok {
				continue
			}
		}
		score := subtitle.ScoreSubtitle(stem, p, "", nil, false, false)
		if score < minSidecarScore {
			continue
		}
		flags := strings.TrimSuffix(path.Base(p), path.Ext(p))[len(stem):]
		f, ok := parser.ParseExternalFile(naming.ExternalSubtitle, languages(), p, flags)
		if !ok {
			continue
		}
		found = append(found, candidate{
			stream: core.MediaStream{
				Kind: core.StreamSubtitle, Codec: codec, Language: f.Language, Title: f.Title,
				Default: f.IsDefault, Forced: f.IsForced, HearingImpaired: f.IsHearingImpaired, ExternalPath: p,
			},
			score: subtitle.ScoreSubtitle(stem, p, f.Language, nil, f.IsForced, f.IsHearingImpaired),
		})
	}
	slices.SortFunc(found, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.stream.ExternalPath, b.stream.ExternalPath))
	})
	out := make([]core.MediaStream, len(found))
	for i, c := range found {
		out[i] = c.stream
	}
	return out
}

// withSidecars returns the embedded streams of streams followed by
// sidecars, numbered after the embedded ones.
func withSidecars(streams, sidecars []core.MediaStream) []core.MediaStream {
	out := slices.DeleteFunc(slices.Clone(streams), func(s core.MediaStream) bool { return s.ExternalPath != "" })
	next := 0
	for _, s := range out {
		next = max(next, s.Index+1)
	}
	for _, s := range sidecars {
		s.Index = next
		next++
		out = append(out, s)
	}
	return out
}

// sidecarsOf returns the external streams of streams.
func sidecarsOf(streams []core.MediaStream) []core.MediaStream {
	var out []core.MediaStream
	for _, s := range streams {
		if s.ExternalPath != "" {
			out = append(out, s)
		}
	}
	return out
}

// sameSidecars reports whether two lists name the same files alike.
func sameSidecars(a, b []core.MediaStream) bool {
	return slices.EqualFunc(a, b, func(x, y core.MediaStream) bool {
		return x.ExternalPath == y.ExternalPath && x.Codec == y.Codec && x.Language == y.Language && x.Title == y.Title &&
			x.Default == y.Default && x.Forced == y.Forced && x.HearingImpaired == y.HearingImpaired
	})
}
