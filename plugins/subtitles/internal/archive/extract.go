package archive

import (
	"errors"
	"path"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// WantFor returns which subtitle of an archive a video wants in a
// language.
func WantFor(v *provider.Video, l provider.Language) Want {
	w := Want{Forced: l.Forced}
	if v != nil && v.Episode {
		w.Episode = v.Number
	}
	return w
}

// Extract returns the wanted subtitle of a downloaded archive or
// subtitle file. Content that is neither, or an archive without the
// subtitle, is a bad subtitle, to be blocklisted.
func Extract(data []byte, w Want, l provider.Language) (provider.File, error) {
	a, err := Open(data)
	if errors.Is(err, ErrNotArchive) {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "the download is not a subtitle or an archive")
	}
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	name, sub, err := Subtitle(a, w)
	if err != nil {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "%w", err)
	}
	if name == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no subtitle of the video in the archive")
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if format == "sub" || format == "" {
		format = Detect(sub)
	}
	return provider.File{Data: sub, Format: format, Language: l}, nil
}
