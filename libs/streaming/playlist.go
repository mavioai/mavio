package streaming

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// MediaPlaylist is a VOD media playlist of a whole media source.
type MediaPlaylist struct {
	Segments []time.Duration
	// Container is the segment container, "mp4" (fMP4) or "ts".
	Container string
	// URI returns the URI of a segment; index -1 is the fMP4
	// initialization segment.
	URI func(index int) string
}

// WriteTo writes the playlist.
func (p MediaPlaylist) WriteTo(w io.Writer) (int64, error) {
	var b strings.Builder
	fmp4 := p.Container != "ts"
	version := 3
	if fmp4 {
		version = 7
	}
	var longest time.Duration
	for _, s := range p.Segments {
		longest = max(longest, s)
	}
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-VERSION:%d\n", version)
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", int(math.Ceil(longest.Seconds())))
	if fmp4 {
		fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", p.URI(-1))
	}
	for i, s := range p.Segments {
		fmt.Fprintf(&b, "#EXTINF:%s,\n%s\n", strconv.FormatFloat(s.Seconds(), 'f', 6, 64), p.URI(i))
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	n, err := io.WriteString(w, b.String())
	return int64(n), err
}

// Variant is a stream of a master playlist.
type Variant struct {
	URI string
	// Bandwidth is the peak bit rate; required.
	Bandwidth int
	// Codecs are RFC 6381 codec strings; SupplementalCodecs names the
	// Dolby Vision or HDR10+ layer clients may use, e.g. "dvh1.08.06/db1p".
	Codecs             []string
	SupplementalCodecs string
	Width, Height      int
	FrameRate          float64
	// VideoRange is "SDR", "PQ" or "HLG"; empty without video.
	VideoRange string
	// Subtitles names the group of subtitle renditions.
	Subtitles string
}

// Subtitle is a subtitle rendition, a WebVTT playlist.
type Subtitle struct {
	Group, Name, Language, URI string
	Default, Forced            bool
}

// MasterPlaylist lists the variants of a stream and its subtitles.
type MasterPlaylist struct {
	Variants  []Variant
	Subtitles []Subtitle
}

// WriteTo writes the playlist.
func (p MasterPlaylist) WriteTo(w io.Writer) (int64, error) {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, s := range p.Subtitles {
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=%q,NAME=%q", s.Group, s.Name)
		if s.Language != "" {
			fmt.Fprintf(&b, ",LANGUAGE=%q", s.Language)
		}
		fmt.Fprintf(&b, ",DEFAULT=%s,AUTOSELECT=YES,FORCED=%s,URI=%q\n", yesNo(s.Default), yesNo(s.Forced), s.URI)
	}
	for _, v := range p.Variants {
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d", v.Bandwidth, v.Bandwidth)
		if v.VideoRange != "" {
			b.WriteString(",VIDEO-RANGE=" + v.VideoRange)
		}
		if len(v.Codecs) > 0 {
			fmt.Fprintf(&b, ",CODECS=%q", strings.Join(v.Codecs, ","))
		}
		if v.SupplementalCodecs != "" {
			fmt.Fprintf(&b, ",SUPPLEMENTAL-CODECS=%q", v.SupplementalCodecs)
		}
		if v.Width > 0 && v.Height > 0 {
			fmt.Fprintf(&b, ",RESOLUTION=%dx%d", v.Width, v.Height)
		}
		if v.FrameRate > 0 {
			b.WriteString(",FRAME-RATE=" + strconv.FormatFloat(v.FrameRate, 'f', 3, 64))
		}
		if v.Subtitles != "" {
			fmt.Fprintf(&b, ",SUBTITLES=%q", v.Subtitles)
		}
		b.WriteString("\n" + v.URI + "\n")
	}
	n, err := io.WriteString(w, b.String())
	return int64(n), err
}

func yesNo(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}
