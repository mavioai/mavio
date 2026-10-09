package providers

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/probe"
)

// FFprobe probes media files with ffprobe as a library.Prober.
type FFprobe struct {
	Prober *probe.Prober
}

// Probe probes the file at path, with its chapters.
func (f FFprobe) Probe(ctx context.Context, path string, audio bool) (library.ProbeResult, error) {
	r, err := f.Prober.Probe(ctx, probe.Request{Path: path, Audio: audio, Chapters: true})
	if err != nil {
		return library.ProbeResult{}, err
	}
	// Attached files, such as the fonts of ASS subtitles, are streams too;
	// attached pictures are cover art, not files.
	for _, a := range r.Attachments {
		if a.FileName == "" && a.MimeType == "" {
			continue
		}
		r.Source.Streams = append(r.Source.Streams, core.MediaStream{
			Index: a.Index, Kind: core.StreamAttachment, Codec: a.Codec, Title: a.FileName, MimeType: a.MimeType,
		})
	}
	return library.ProbeResult{Source: r.Source, Tags: r.Metadata}, nil
}
