package release

import (
	"maps"
	"strings"
)

// sdExtensions are the media file extensions both Sonarr and Radarr map to
// SDTV.
var sdExtensions = []string{
	".m4v", ".3gp", ".nsv", ".ty", ".strm", ".rm", ".rmvb", ".m3u", ".ifo", ".mov", ".qt", ".divx", ".xvid", ".bivx",
	".nrg", ".pva", ".wmv", ".asf", ".asx", ".ogm", ".ogv", ".m2v", ".avi", ".bin", ".dat", ".dvr-ms", ".mpg", ".mpeg",
	".mp4", ".avc", ".vp3", ".svq3", ".nuv", ".viv", ".dv", ".fli", ".flv", ".wpl",
}

func extensionTable(extra map[string]Quality) map[string]Quality {
	t := map[string]Quality{".webm": Unknown, ".img": DVD, ".iso": DVD, ".vob": DVD, ".m2ts": Bluray720p}
	for _, e := range sdExtensions {
		t[e] = SDTV
	}
	maps.Copy(t, extra)
	return t
}

var extensionQualities = map[Kind]map[string]Quality{
	Movie:  extensionTable(map[string]Quality{".mkv": WEBDL720p, ".mk3d": WEBDL720p, ".ts": SDTV, ".wtv": SDTV}),
	Series: extensionTable(map[string]Quality{".mkv": HDTV720p, ".ts": HDTV720p, ".wtv": HDTV720p}),
}

// IsMediaExtension reports whether ext (with the leading dot, any case) is a
// media file extension.
func IsMediaExtension(ext string, kind Kind) bool {
	_, ok := extensionQualities[kind][strings.ToLower(ext)]
	return ok
}

// QualityForExtension returns the quality implied by a media file extension
// (with the leading dot, any case), or Unknown.
func QualityForExtension(ext string, kind Kind) Quality {
	return extensionQualities[kind][strings.ToLower(ext)]
}

// pathExtension returns the extension of the last path element of name,
// including the dot, like .NET's Path.GetExtension.
func pathExtension(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		switch name[i] {
		case '.':
			if i == len(name)-1 {
				return ""
			}
			return name[i:]
		case '/', '\\':
			return ""
		}
	}
	return ""
}
