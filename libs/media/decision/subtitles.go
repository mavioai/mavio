package decision

import (
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// SubtitleProfileFor returns how a subtitle stream of s reaches the client
// when played with method into outputContainer over protocol (empty when
// not transcoding): embedded when the output can carry it, else as a
// separate file or HLS rendition, converting text formats if needed, else
// burned in.
func SubtitleProfileFor(s *Source, st *core.MediaStream, profiles []SubtitleProfile, method Method, t Transcoder, outputContainer string, protocol Protocol) SubtitleProfile {
	return subtitleProfile(s, st, profiles, method, t, outputContainer, protocol)
}

func subtitleProfile(s *Source, st *core.MediaStream, profiles []SubtitleProfile, method Method, t Transcoder, outputContainer string, protocol Protocol) SubtitleProfile {
	if canEmbedSubtitle(st, method, protocol, outputContainer) {
		embeddable := func(p *SubtitleProfile) bool {
			return p.supportsLanguage(st.Language) && p.Method == SubtitleEmbed &&
				p.Container.Contains(outputContainer) &&
				(method != Transcode || embedsSubtitles(outputContainer))
		}
		// The same format first, then one it converts to.
		for i := range profiles {
			p := &profiles[i]
			if embeddable(p) && st.IsTextSubtitle() == core.IsTextSubtitleCodec(p.Format) && strings.EqualFold(p.Format, st.Codec) {
				return *p
			}
		}
		for i := range profiles {
			p := &profiles[i]
			if embeddable(p) && st.IsTextSubtitle() && canConvertSubtitle(st, p.Format) {
				return *p
			}
		}
	}
	if p := externalSubtitleProfile(s, st, profiles, method, t, false); p != nil {
		return *p
	}
	if p := externalSubtitleProfile(s, st, profiles, method, t, true); p != nil {
		return *p
	}
	return SubtitleProfile{Method: SubtitleEncode, Format: st.Codec}
}

// embedsSubtitles reports whether a transcoding container carries
// subtitles: Matroska does, MPEG-TS and MP4 do not here.
func embedsSubtitles(container string) bool {
	if container == "" {
		return false
	}
	if Names(container).Contains("ts,mpegts,mp4") {
		return false
	}
	return Names(container).Contains("mkv,matroska")
}

// canEmbedSubtitle reports whether a stream may be embedded: external
// files only into progressive transcodes that carry subtitles, embedded
// streams unless transcoding to HLS.
func canEmbedSubtitle(st *core.MediaStream, method Method, protocol Protocol, outputContainer string) bool {
	if isExternal(st) {
		return method == Transcode && protocol != HLS && embedsSubtitles(outputContainer)
	}
	return method != Transcode || protocol != HLS
}

func externalSubtitleProfile(s *Source, st *core.MediaStream, profiles []SubtitleProfile, method Method, t Transcoder, allowConversion bool) *SubtitleProfile {
	for i := range profiles {
		p := &profiles[i]
		if p.Method != SubtitleExternal && p.Method != SubtitleHLS {
			continue
		}
		if p.Method == SubtitleHLS && method != Transcode {
			continue
		}
		if !p.supportsLanguage(st.Language) {
			continue
		}
		// Embedded streams must be extracted while transcoding; PGS and
		// VobSub always can be.
		if !isExternal(st) && method == Transcode && (t == nil || !t.CanExtractSubtitles(st.Codec)) &&
			!st.IsPGSSubtitle() && !st.IsVobSubSubtitle() {
			continue
		}
		vobSubMKS := isVobSubMKSProfile(p, st)
		matches := (p.Method == SubtitleExternal &&
			(vobSubMKS || (!isVobSubMKSDelivery(p) && st.IsTextSubtitle() == core.IsTextSubtitleCodec(p.Format)))) ||
			(p.Method == SubtitleHLS && st.IsTextSubtitle())
		if !matches {
			continue
		}
		if vobSubMKS || strings.EqualFold(st.Codec, p.Format) {
			return p
		}
		if !allowConversion || s.Infinite {
			continue
		}
		if st.IsTextSubtitle() && supportsExternalStream(st) && canConvertSubtitle(st, p.Format) {
			return p
		}
	}
	return nil
}

// isVobSubMKSDelivery reports whether a profile takes VobSub in Matroska
// subtitle files, the form ffmpeg extracts VobSub to.
func isVobSubMKSDelivery(p *SubtitleProfile) bool {
	return core.IsVobSubSubtitleCodec(p.Format) && strings.TrimSpace(string(p.Container)) != "" && p.Container.Contains("mks")
}

// isVobSubMKSProfile reports whether such a profile fits a VobSub stream:
// an embedded one, which is extracted to .mks, or an external .mks file.
func isVobSubMKSProfile(p *SubtitleProfile, st *core.MediaStream) bool {
	return isVobSubMKSDelivery(p) && st.IsVobSubSubtitle() &&
		(!isExternal(st) || strings.HasSuffix(strings.ToLower(st.ExternalPath), ".mks"))
}
