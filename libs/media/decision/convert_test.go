package decision

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// The ported StreamBuilder cases describe clients and sources in Jellyfin's
// DeviceProfile and MediaSourceInfo JSON; these helpers convert them.

// flexInt reads a number written as a number or a string.
type flexInt int

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(strings.Trim(string(b), `"`))
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.Atoi(s)
	*n = flexInt(v)
	return err
}

type jfCondition struct {
	Condition  string
	Property   string
	Value      string
	IsRequired *bool
}

type jfProfile struct {
	Name                             string
	MaxStreamingBitrate              *int64
	MaxStaticBitrate                 *int64
	MusicStreamingTranscodingBitrate *int64
	MaxStaticMusicBitrate            *int64
	DirectPlayProfiles               []struct{ Container, Type, VideoCodec, AudioCodec string }
	TranscodingProfiles              []struct {
		Container, Type, VideoCodec, AudioCodec, Protocol, Context, TranscodeSeekInfo          string
		EstimateContentLength, EnableMpegtsM2TsMode, CopyTimestamps, EnableSubtitlesInManifest bool
		EnableAudioVbrEncoding                                                                 *bool
		MaxAudioChannels, MinSegments, SegmentLength                                           flexInt
		Conditions                                                                             []jfCondition
	}
	ContainerProfiles []struct {
		Type, Container, SubContainer string
		Conditions                    []jfCondition
	}
	CodecProfiles []struct {
		Type, Codec, Container, SubContainer string
		Conditions, ApplyConditions          []jfCondition
	}
	SubtitleProfiles []struct{ Format, Method, Language, Container string }
}

// jfRangeTypes maps Jellyfin's range type names to Mavio's.
var jfRangeTypes = map[string]core.VideoRangeType{
	"sdr": core.RangeTypeSDR, "hdr10": core.RangeTypeHDR10, "hdr10plus": core.RangeTypeHDR10Plus,
	"hlg": core.RangeTypeHLG, "dovi": core.RangeTypeDOVI, "doviwithhdr10": core.RangeTypeDOVIWithHDR10,
	"doviwithhlg": core.RangeTypeDOVIWithHLG, "doviwithsdr": core.RangeTypeDOVIWithSDR,
	"doviwithel": core.RangeTypeDOVIWithEL, "doviwithhdr10plus": core.RangeTypeDOVIWithHDR10Plus,
	"doviwithelhdr10plus": core.RangeTypeDOVIWithELHDR10Plus, "doviinvalid": core.RangeTypeDOVIInvalid,
}

func convertConditions(in []jfCondition) []Condition {
	var out []Condition
	for _, c := range in {
		value := c.Value
		if c.Property == "VideoRangeType" {
			parts := strings.Split(value, "|")
			for i, p := range parts {
				if t, ok := jfRangeTypes[strings.ToLower(strings.TrimSpace(p))]; ok {
					parts[i] = string(t)
				}
			}
			value = strings.Join(parts, "|")
		}
		out = append(out, Condition{
			Property: Property(c.Property),
			Op:       Op(c.Condition),
			Value:    value,
			Optional: c.IsRequired != nil && !*c.IsRequired,
		})
	}
	return out
}

func mediaKind(t string) MediaKind { return MediaKind(strings.ToLower(t)) }

func codecKind(t string) CodecKind {
	switch t {
	case "Audio":
		return CodecAudio
	case "VideoAudio":
		return CodecVideoAudio
	}
	return CodecVideo
}

func orDefault(v *int64, def int64) int64 {
	if v == nil {
		return def
	}
	return *v
}

func convertProfile(p *jfProfile) *ClientCapabilities {
	c := &ClientCapabilities{
		Name:                             p.Name,
		MaxStreamingBitrate:              orDefault(p.MaxStreamingBitrate, 8_000_000),
		MaxStaticBitrate:                 orDefault(p.MaxStaticBitrate, 8_000_000),
		MusicStreamingTranscodingBitrate: orDefault(p.MusicStreamingTranscodingBitrate, 128_000),
		MaxStaticMusicBitrate:            orDefault(p.MaxStaticMusicBitrate, 8_000_000),
	}
	for _, d := range p.DirectPlayProfiles {
		c.DirectPlay = append(c.DirectPlay, DirectPlayProfile{
			Kind: mediaKind(d.Type), Container: Names(d.Container), VideoCodec: Names(d.VideoCodec), AudioCodec: Names(d.AudioCodec),
		})
	}
	for _, t := range p.TranscodingProfiles {
		protocol := Protocol(strings.ToLower(t.Protocol))
		if protocol == "" {
			protocol = HTTP
		}
		c.Transcoding = append(c.Transcoding, TranscodingProfile{
			Kind: mediaKind(t.Type), Context: Context(strings.ToLower(cmpOr(t.Context, "Streaming"))), Protocol: protocol,
			Container: t.Container, VideoCodec: Names(t.VideoCodec), AudioCodec: Names(t.AudioCodec),
			MaxAudioChannels: int(t.MaxAudioChannels), EstimateContentLength: t.EstimateContentLength,
			EnableMpegtsM2TsMode: t.EnableMpegtsM2TsMode, SeekInfo: SeekInfo(strings.ToLower(cmpOr(t.TranscodeSeekInfo, "Auto"))),
			CopyTimestamps: t.CopyTimestamps, EnableSubtitlesInManifest: t.EnableSubtitlesInManifest,
			MinSegments: int(t.MinSegments), SegmentLength: int(t.SegmentLength),
			Conditions:      convertConditions(t.Conditions),
			DisableAudioVBR: t.EnableAudioVbrEncoding != nil && !*t.EnableAudioVbrEncoding,
		})
	}
	for _, k := range p.ContainerProfiles {
		c.Containers = append(c.Containers, ContainerProfile{
			Kind: mediaKind(k.Type), Container: Names(k.Container), SubContainer: Names(k.SubContainer), Conditions: convertConditions(k.Conditions),
		})
	}
	for _, k := range p.CodecProfiles {
		c.Codecs = append(c.Codecs, CodecProfile{
			Kind: codecKind(k.Type), Codec: Names(k.Codec), Container: Names(k.Container), SubContainer: Names(k.SubContainer),
			Conditions: convertConditions(k.Conditions), ApplyConditions: convertConditions(k.ApplyConditions),
		})
	}
	for _, s := range p.SubtitleProfiles {
		c.Subtitles = append(c.Subtitles, SubtitleProfile{
			Format: s.Format, Method: SubtitleMethod(strings.ToLower(s.Method)), Language: Names(s.Language), Container: Names(s.Container),
		})
	}
	return c
}

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// jfStream is a Jellyfin MediaStream.
type jfStream struct {
	Type                                         json.RawMessage
	Index                                        int
	Codec, CodecTag, Profile, Language, Path     string
	Level                                        float64
	BitRate, BitDepth, RefFrames, Width          int
	Height, Channels, SampleRate, Rotation       int
	AverageFrameRate, RealFrameRate              json.Number
	ColorTransfer, ColorPrimaries, ColorSpace    string
	PixelFormat                                  string
	IsDefault, IsForced, IsExternal              bool
	IsInterlaced, IsAnamorphic, IsAVC            bool
	IsHearingImpaired                            bool
	Score                                        *int
	DvProfile, DvLevel                           int
	DvBlSignalCompatibilityID                    int `json:"DvBlSignalCompatibilityId"`
	DvVersionMajor, DvVersionMinor               int
	RpuPresentFlag, ElPresentFlag, BlPresentFlag int
}

var jfStreamKinds = map[string]core.StreamKind{
	"0": core.StreamAudio, "1": core.StreamVideo, "2": core.StreamSubtitle, "3": core.StreamImage, "4": core.StreamData,
	`"Audio"`: core.StreamAudio, `"Video"`: core.StreamVideo, `"Subtitle"`: core.StreamSubtitle,
	`"EmbeddedImage"`: core.StreamImage, `"Data"`: core.StreamData,
}

// rational converts a decimal frame rate exactly.
func rational(n json.Number) core.Rational {
	if n == "" {
		return core.Rational{}
	}
	r, ok := new(big.Rat).SetString(string(n))
	if !ok || !r.Num().IsInt64() || !r.Denom().IsInt64() {
		return core.Rational{}
	}
	return core.Rational{Num: r.Num().Int64(), Den: r.Denom().Int64()}
}

func convertStream(s *jfStream) core.MediaStream {
	kind := core.StreamAudio // Jellyfin's default stream type
	if len(s.Type) > 0 {
		kind = jfStreamKinds[string(s.Type)]
	}
	st := core.MediaStream{
		Index: s.Index, Kind: kind, Codec: s.Codec, CodecTag: s.CodecTag, Profile: s.Profile,
		Level: int(s.Level), Bitrate: int64(s.BitRate), Language: s.Language,
		Default: s.IsDefault, Forced: s.IsForced, HearingImpaired: s.IsHearingImpaired,
		Width: s.Width, Height: s.Height, FrameRate: rational(s.AverageFrameRate), RealFrameRate: rational(s.RealFrameRate),
		PixelFormat: s.PixelFormat, BitDepth: s.BitDepth, ColorPrimaries: s.ColorPrimaries,
		ColorTransfer: s.ColorTransfer, ColorSpace: s.ColorSpace, Interlaced: s.IsInterlaced, Rotation: s.Rotation,
		Anamorphic: s.IsAnamorphic, AVC: s.IsAVC, RefFrames: s.RefFrames, Channels: s.Channels, SampleRate: s.SampleRate,
	}
	if s.IsExternal {
		// Mavio marks external streams by their path.
		st.ExternalPath = cmpOr(s.Path, "external")
	}
	if s.DvProfile != 0 || s.RpuPresentFlag != 0 || s.BlPresentFlag != 0 {
		st.DolbyVision = &core.DolbyVision{
			VersionMajor: s.DvVersionMajor, VersionMinor: s.DvVersionMinor, Profile: s.DvProfile, Level: s.DvLevel,
			RPUPresent: s.RpuPresentFlag == 1, ELPresent: s.ElPresentFlag == 1, BLPresent: s.BlPresentFlag == 1,
			BLCompatibilityID: s.DvBlSignalCompatibilityID,
		}
	}
	return st
}

type jfSource struct {
	ID                                                  string `json:"Id"`
	Path, Container, Protocol, VideoType                string
	Bitrate                                             int64
	IsRemote, IsInfiniteStream                          bool
	SupportsDirectPlay, SupportsDirectStream            *bool
	SupportsTranscoding                                 *bool
	DefaultAudioStreamIndex, DefaultSubtitleStreamIndex *int
	MediaStreams                                        []jfStream
}

// sourceIDs gives each Jellyfin source ID a stable Mavio ID.
func sourceID(id string) core.ID {
	var b [16]byte
	copy(b[:], id)
	return core.ID(b)
}

func convertSource(j *jfSource) *Source {
	ms := &core.MediaSource{ID: sourceID(j.ID), Path: j.Path, Container: j.Container, Bitrate: j.Bitrate}
	s := &Source{
		MediaSource:          ms,
		Remote:               j.IsRemote || (j.Protocol != "" && j.Protocol != "File"),
		Disc:                 j.VideoType == "Dvd" || j.VideoType == "BluRay",
		Infinite:             j.IsInfiniteStream,
		NoDirectPlay:         j.SupportsDirectPlay != nil && !*j.SupportsDirectPlay,
		NoDirectStream:       j.SupportsDirectStream != nil && !*j.SupportsDirectStream,
		NoTranscoding:        j.SupportsTranscoding != nil && !*j.SupportsTranscoding,
		DefaultAudioIndex:    j.DefaultAudioStreamIndex,
		DefaultSubtitleIndex: j.DefaultSubtitleStreamIndex,
	}
	for i := range j.MediaStreams {
		ms.Streams = append(ms.Streams, convertStream(&j.MediaStreams[i]))
		if sc := j.MediaStreams[i].Score; sc != nil {
			if s.SubtitleScores == nil {
				s.SubtitleScores = map[int]int{}
			}
			s.SubtitleScores[j.MediaStreams[i].Index] = *sc
		}
	}
	return s
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func loadProfile(t *testing.T, name string) *ClientCapabilities {
	t.Helper()
	var p jfProfile
	readJSON(t, filepath.Join("testdata", "device_profiles", "DeviceProfile-"+name+".json"), &p)
	return convertProfile(&p)
}

func loadSource(t *testing.T, name string) *Source {
	t.Helper()
	var s jfSource
	readJSON(t, filepath.Join("testdata", "media_sources", "MediaSourceInfo-"+name+".json"), &s)
	return convertSource(&s)
}
