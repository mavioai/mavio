package probe

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mavioai/mavio/libs/core"
)

// Result is a probed media file: the media source facts and the metadata
// found in its tags.
type Result struct {
	Source core.MediaSource
	// Attachments are fonts and other files embedded in the container.
	Attachments []Attachment
	// Metadata holds the item fields found in tags: name, sort name,
	// overview, numbering, year and premiere date, album and artists,
	// genres, studios, rating, MusicBrainz IDs, runtime and 3D format.
	Metadata core.Item
	// People are credits found in tags, such as composers.
	People []Person
	// SeriesName is the show an episode file names in its tags.
	SeriesName string
}

// Attachment is a file embedded in the container, or attached cover art.
type Attachment struct {
	Index    int
	Codec    string
	CodecTag string
	FileName string
	MimeType string
	Comment  string
}

// Person is a credit found in tags.
type Person struct {
	Name string
	Kind core.CreditKind
	Role string
}

var (
	basicDelimiters = "/;"
	nameDelimiters  = "/;|\\"
	genreDelimiters = "/;,"

	webmVideoCodecs = []string{"av1", "vp8", "vp9"}
	webmAudioCodecs = []string{"opus", "vorbis"}

	// splitWhitelist are artist names containing delimiters.
	splitWhitelist = []string{
		"AC/DC", "A/T/O/S", "As/Hi Soundworks", "Au/Ra", "Bremer/McCoy", "b/bqスタヂオ", "DOV/S",
		"DJ'TEKINA//SOMETHING", "IX/ON", "J-CORE SLi//CER", "M(a/u)SH", "Kaoru/Brilliance", "signum/ii",
		"Richiter(LORB/DUGEM DI BARAT)", "이달의 소녀 1/3", "R!N / Gemie", "LOONA 1/3", "LOONA / yyxy",
		"LOONA / ODD EYE CIRCLE", "K/DA", "22/7", "諭吉佳作/men", "//dARTH nULL", "Phantom/Ghost",
		"She/Her/Hers", "5/8erl in Ehr'n", "Smith/Kotzen", "We;Na", "LSR/CITY", "Kairon; IRSE!",
	}

	performerRegex       = regexp.MustCompile(`(?P<name>.*) \((?P<instrument>.*)\)`)
	durationOverPrecison = regexp.MustCompile(`(\.\d{7})\d+`)
)

// Normalize turns ffprobe's output for the file at path into a Result, as
// Jellyfin does. isAudio selects audio-file handling: the first audio
// stream's tags, audio runtime and credits.
func Normalize(out *Output, path string, isAudio bool) Result {
	var r Result
	src := &r.Source
	src.Path = path
	if out.Format != nil && out.Format.Size != "" {
		src.Size, _ = strconv.ParseInt(out.Format.Size, 10, 64)
	}
	var formatBitrate string
	if out.Format != nil {
		formatBitrate = out.Format.BitRate
	}
	for i := range out.Streams {
		s := &out.Streams[i]
		if a, ok := attachment(s); ok {
			r.Attachments = append(r.Attachments, a)
		}
		st, ok := stream(isAudio, s, formatBitrate, out.Frames)
		// Subtitles without a known codec would only cause failures.
		if !ok || (st.Kind == core.StreamSubtitle && strings.TrimSpace(st.Codec) == "") {
			continue
		}
		src.Streams = append(src.Streams, st)
	}
	if out.Format != nil {
		src.Container = normalizeFormat(out.Format.FormatName, src.Streams)
		if v, err := strconv.ParseInt(out.Format.BitRate, 10, 32); err == nil {
			src.Bitrate = v
		}
	}

	tags := Tags{}
	tagType := "video"
	if isAudio {
		tagType = "audio"
	}
	for _, s := range out.Streams {
		if s.CodecType == tagType {
			for k, v := range s.Tags {
				tags[k] = v
			}
			break
		}
	}
	if out.Format != nil {
		for k, v := range out.Format.Tags {
			tags[k] = v
		}
	}

	md := &r.Metadata
	md.Genres = genres(md.Genres, tags)
	md.Name = tags.firstNonBlank("title", "title-eng")
	md.SortName = tags.firstNonBlank("sort_name", "title-sort", "titlesort")
	md.Overview = tags.firstNonBlank("synopsis", "description", "desc", "comment")
	md.ParentIndexNumber = numericTag(tags, "season_number")
	if md.IndexNumber = numericTag(tags, "episode_sort"); md.IndexNumber == nil {
		md.IndexNumber = numericTag(tags, "episode_id")
	}
	r.SeriesName, _ = tags.get("show_name", "show")
	if y := numericTag(tags, "date"); y != nil {
		md.ProductionYear = *y
	}
	for _, key := range []string{"originaldate", "retaildate", "retail date", "retail_date", "date_released", "date", "creation_time"} {
		if t, ok := dateTag(tags, key); ok {
			md.PremiereDate = &t
			break
		}
	}
	md.Album, _ = tags.get("album")
	if artists, ok := tags.get("artists"); ok && strings.TrimSpace(artists) != "" {
		md.Artists = splitDistinctArtists(artists, basicDelimiters, false)
	} else if artist := tags.firstNonBlank("artist"); artist != "" {
		md.Artists = splitDistinctArtists(artist, nameDelimiters, true)
	}
	if md.ProductionYear == 0 && md.PremiereDate != nil {
		md.ProductionYear = md.PremiereDate.Year()
	}
	for _, c := range out.Chapters {
		src.Chapters = append(src.Chapters, chapter(c))
	}

	if isAudio {
		audioRuntime(out, &r)
		audioTags(&r, tags)
		return r
	}

	fetchStudios(md, tags, "copyright")
	if extc := tags.firstNonBlank("iTunEXTC"); extc != "" {
		// e.g. "mpaa|G|100|For crude humor"
		if parts := splitNonEmpty(extc, "|"); len(parts) > 1 {
			md.OfficialRating = parts[1]
		}
	}
	if movi := tags.firstNonBlank("iTunMOVI"); movi != "" {
		itunesInfo(movi, &r)
	}
	if out.Format != nil && out.Format.Duration != "" {
		if secs, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
			src.Duration = seconds(secs)
		}
	}
	md.Runtime = src.Duration
	wtvInfo(&r, out)
	if mode, ok := tags.get("stereo_mode"); ok && strings.EqualFold(mode, "left_right") {
		md.Video3DFormat = core.Video3DFullSideBySide
	}

	for i := range src.Streams {
		s := &src.Streams[i]
		if s.Kind == core.StreamAudio && s.Bitrate == 0 {
			if b, ok := EstimatedAudioBitrate(s.Codec, s.Profile, s.Channels); ok {
				s.Bitrate = int64(b)
			}
		}
	}
	// ffprobe often omits the video bitrate: estimate it as the container
	// bitrate minus the other streams', when every audio bitrate is known.
	var videos []*core.MediaStream
	for i := range src.Streams {
		if src.Streams[i].Kind == core.StreamVideo {
			videos = append(videos, &src.Streams[i])
		}
	}
	if src.Bitrate > 0 && len(videos) == 1 && videos[0].Bitrate == 0 {
		var others int64
		known := true
		for _, s := range src.Streams {
			if s.Kind == core.StreamVideo || s.ExternalPath != "" {
				continue
			}
			if s.Kind == core.StreamAudio && s.Bitrate == 0 {
				known = false
			}
			others += s.Bitrate
		}
		if known && src.Bitrate-others > 0 {
			videos[0].Bitrate = src.Bitrate - others
		}
	}
	// Infer the container bitrate from the streams if unknown.
	if src.Bitrate == 0 {
		var sum int64
		for _, s := range src.Streams {
			sum += s.Bitrate
		}
		src.Bitrate = sum
	}
	return r
}

func seconds(s float64) time.Duration { return time.Duration(math.Round(s * float64(time.Second))) }

// normalizeFormat names Matroska "mkv", MPEG-TS "ts" and MPEG-1 "mpeg",
// and keeps "webm" only when every stream is WebM-compatible.
func normalizeFormat(format string, streams []core.MediaStream) string {
	if strings.TrimSpace(format) == "" {
		return ""
	}
	var out []string
	for f := range strings.SplitSeq(format, ",") {
		switch strings.ToLower(f) {
		case "mpegvideo":
			f = "mpeg"
		case "mpegts":
			f = "ts"
		case "matroska":
			f = "mkv"
		case "webm":
			for _, s := range streams {
				switch {
				case s.Kind != core.StreamVideo && s.Kind != core.StreamAudio,
					s.Kind == core.StreamVideo && !containsFold(webmVideoCodecs, s.Codec),
					s.Kind == core.StreamAudio && !containsFold(webmAudioCodecs, s.Codec):
					f = ""
				}
			}
		}
		if f != "" {
			out = append(out, f)
		}
	}
	return strings.Join(out, ",")
}

// EstimatedAudioBitrate returns a typical bitrate for an audio codec,
// profile and channel count, used when ffprobe reports none.
func EstimatedAudioBitrate(codec, profile string, channels int) (int, bool) {
	if channels < 1 || codec == "" {
		return 0, false
	}
	multi := channels > 2
	pick := func(multichannel, stereo int) (int, bool) {
		if multi {
			return multichannel, true
		}
		return stereo, true
	}
	switch strings.ToLower(codec) {
	case "aac", "mp3", "mp2":
		return pick(320000, 192000)
	case "ac3", "eac3":
		return pick(640000, 192000)
	case "dts", "dca":
		if strings.Contains(strings.ToLower(profile), "hd ma") {
			return channels * 700000, true
		}
		return pick(1509000, 768000)
	case "opus":
		return pick(256000, 128000)
	case "vorbis":
		return pick(320000, 160000)
	case "wmav1", "wmav2", "wmapro":
		return pick(384000, 192000)
	case "flac", "alac":
		return channels * 480000, true
	case "truehd", "mlp":
		return channels * 700000, true
	}
	return 0, false
}

func normalizeSubtitleCodec(codec string) string {
	switch strings.ToLower(codec) {
	case "dvb_subtitle":
		return "DVBSUB"
	case "dvb_teletext":
		return "DVBTXT"
	case "dvd_subtitle":
		return "DVDSUB" // .sub + .idx
	case "hdmv_pgs_subtitle":
		return "PGSSUB" // .sup
	}
	return codec
}

func attachment(s *Stream) (Attachment, bool) {
	if s.CodecType != "attachment" && s.Disposition["attached_pic"] != 1 {
		return Attachment{}, false
	}
	a := Attachment{Index: s.Index, Codec: s.CodecName, CodecTag: strings.TrimSpace(s.CodecTagString)}
	a.FileName, a.MimeType, a.Comment = s.Tags["filename"], s.Tags["mimetype"], s.Tags["comment"]
	return a, true
}

// stream normalizes one stream; it reports false for streams that are not
// video, audio, subtitles or data.
func stream(isAudio bool, s *Stream, formatBitrate string, frames []Frame) (core.MediaStream, bool) {
	st := core.MediaStream{
		Index:         s.Index,
		Codec:         s.CodecName,
		Profile:       s.Profile,
		PixelFormat:   s.PixelFormat,
		NALLengthSize: string(s.NALLengthSize),
		TimeBase:      s.TimeBase,
		CodecTimeBase: s.CodecTimeBase,
	}
	if s.Width != nil {
		st.Width = int(*s.Width)
	}
	if s.Height != nil {
		st.Height = int(*s.Height)
	}
	if s.Level != nil {
		st.Level = int(*s.Level)
	}
	// Codec tags such as "[0][0][0][0]" are junk.
	if t := s.CodecTagString; strings.TrimSpace(t) != "" && !strings.Contains(t, "[0]") {
		st.CodecTag = t
	}
	st.Language, st.Comment, st.Title = s.Tags["language"], s.Tags["comment"], s.Tags["title"]

	// titleFallback uses MP4's name tag, then a handler name other than the
	// default one.
	titleFallback := func(defaultHandler string) {
		if st.Title != "" {
			return
		}
		if st.Title = s.Tags["name"]; st.Title == "" {
			if h := s.Tags["handler_name"]; h != "" && !strings.EqualFold(h, defaultHandler) {
				st.Title = h
			}
		}
	}
	switch s.CodecType {
	case "audio":
		st.Kind = core.StreamAudio
		if s.Channels != nil {
			st.Channels = int(*s.Channels)
		}
		st.SampleRate, _ = strconv.Atoi(s.SampleRate)
		st.ChannelLayout, _, _ = strings.Cut(s.ChannelLayout, "(")
		if s.BitsPerSample > 0 {
			st.BitDepth = int(s.BitsPerSample)
		} else if s.BitsPerRawSample > 0 {
			st.BitDepth = int(s.BitsPerRawSample)
		}
		titleFallback("SoundHandler")
	case "subtitle":
		st.Kind = core.StreamSubtitle
		st.Codec = normalizeSubtitleCodec(st.Codec)
		titleFallback("SubtitleHandler")
	case "video":
		videoStream(isAudio, s, &st, frames)
	case "data":
		st.Kind = core.StreamData
	default:
		return core.MediaStream{}, false
	}

	bitrate, _ := strconv.Atoi(s.BitRate)
	// FLAC files carry their bitrate in the format; for video the format
	// bitrate is the whole container's.
	if bitrate == 0 && isAudio && st.Kind == core.StreamAudio {
		bitrate, _ = strconv.Atoi(formatBitrate)
	}
	if bitrate > 0 {
		st.Bitrate = int64(bitrate)
	}
	if st.Bitrate == 0 && (s.CodecType == "audio" || s.CodecType == "video") {
		if bps := bpsTag(s); bps > 0 {
			st.Bitrate = int64(bps)
		} else if secs, ok := durationTag(s); ok && secs >= 1 {
			if n, ok := bytesTag(s); ok {
				if bps := int64(math.RoundToEven(float64(n) * 8 / secs)); bps > 0 {
					st.Bitrate = bps
				}
			}
		}
	}

	st.Default = s.Disposition["default"] == 1
	st.Forced = s.Disposition["forced"] == 1
	st.HearingImpaired = s.Disposition["hearing_impaired"] == 1
	st.Original = s.Disposition["original"] == 1
	if strings.EqualFold(st.Title, "cc") || st.Kind == core.StreamImage {
		st.Title = ""
	}
	return st, true
}

func videoStream(isAudio bool, s *Stream, st *core.MediaStream, frames []Frame) {
	st.AVC = bool(s.IsAVC)
	st.FrameRate = parseRational(s.AverageFrameRate)
	st.RealFrameRate = parseRational(s.RealFrameRate)
	st.Interlaced = strings.TrimSpace(s.FieldOrder) != "" && !strings.EqualFold(s.FieldOrder, "progressive")
	switch c := strings.ToLower(st.Codec); {
	case isAudio || c == "bmp" || c == "gif" || c == "png" || c == "webp":
		st.Kind = core.StreamImage
	case c == "mjpeg":
		// Embedded images have no codec tag (and unusual frame rates).
		st.Kind = core.StreamImage
		if st.CodecTag != "" {
			st.Kind = core.StreamVideo
		}
	default:
		st.Kind = core.StreamVideo
	}
	st.AspectRatio = aspectRatio(s.DisplayAspectRatio, st.Width, st.Height)
	if s.BitsPerSample > 0 {
		st.BitDepth = int(s.BitsPerSample)
	} else if s.BitsPerRawSample > 0 {
		st.BitDepth = int(s.BitsPerRawSample)
	}
	if st.BitDepth == 0 {
		switch strings.ToLower(s.PixelFormat) {
		case "yuv420p", "yuv444p":
			st.BitDepth = 8
		case "yuv420p10le", "yuv444p10le":
			st.BitDepth = 10
		case "yuv420p12le", "yuv444p12le":
			st.BitDepth = 12
		}
	}
	st.SampleAspectRatio = parseRatio(s.SampleAspectRatio)
	switch {
	case s.SampleAspectRatio == "" && s.DisplayAspectRatio == "":
	case IsNearSquarePixelSAR(s.SampleAspectRatio):
	case s.SampleAspectRatio != "0:1":
		st.Anamorphic = true
	case s.DisplayAspectRatio == "0:1":
	case s.DisplayAspectRatio != aspectRatio("", st.Width, st.Height):
		st.Anamorphic = true
	}
	if s.Refs > 0 {
		st.RefFrames = int(s.Refs)
	}
	st.ColorRange, st.ColorSpace, st.ColorTransfer, st.ColorPrimaries = s.ColorRange, s.ColorSpace, s.ColorTransfer, s.ColorPrimaries
	for _, d := range s.SideData {
		switch strings.ToLower(d.Type) {
		case "dovi configuration record":
			st.DolbyVision = &core.DolbyVision{
				Profile:           int(d.DVProfile),
				Level:             int(d.DVLevel),
				BLCompatibilityID: int(d.DVBLSignalCompatibilityID),
				RPUPresent:        d.RPUPresentFlag == 1,
				ELPresent:         d.ELPresentFlag == 1,
				BLPresent:         d.BLPresentFlag == 1,
				VersionMajor:      int(d.DVVersionMajor),
				VersionMinor:      int(d.DVVersionMinor),
			}
		case "display matrix":
			st.Rotation = int(d.Rotation)
		case "frame cropping":
			// Artificially added cropping is not anamorphism.
			st.Anamorphic = false
		}
	}
	for _, f := range frames {
		if f.StreamIndex != st.Index {
			continue
		}
		for _, d := range f.SideData {
			if strings.EqualFold(d.Type, "HDR Dynamic Metadata SMPTE2094-40 (HDR10+)") {
				st.HDR10Plus = true
			}
		}
		break
	}
}

// aspectRatio names a display aspect ratio, computed from width and
// height when dar is not a valid ratio; it returns dar when no name fits.
func aspectRatio(dar string, width, height int) string {
	w, h := width, height
	if parts := strings.Split(dar, ":"); len(parts) == 2 {
		if pw, err1 := strconv.Atoi(parts[0]); err1 == nil && pw > 0 {
			if ph, err2 := strconv.Atoi(parts[1]); err2 == nil && ph > 0 {
				w, h = pw, ph
			}
		}
	}
	if w > 0 && h > 0 {
		ratio := float64(w) / float64(h)
		for _, c := range []struct {
			value, variance float64
			name            string
		}{
			{1.777777778, .03, "16:9"},
			{1.3333333333, .05, "4:3"},
			{1.41, .005, "1.41:1"},
			{1.5, .005, "1.5:1"},
			{1.6, .005, "1.6:1"},
			{1.66666666667, .005, "5:3"},
			{1.85, .02, "1.85:1"},
			{2.35, .025, "2.35:1"},
			{2.4, .025, "2.40:1"},
		} {
			if math.Abs(ratio-c.value) <= c.variance {
				return c.name
			}
		}
	}
	return dar
}

// IsNearSquarePixelSAR reports whether a sample aspect ratio such as
// "1:1" or "1001:1000" is within 1% of square pixels.
func IsNearSquarePixelSAR(sar string) bool {
	if sar == "" {
		return false
	}
	if parts := strings.Split(sar, ":"); len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			return math.Abs(num/den-1) <= 0.01
		}
	}
	return sar == "1:1"
}

// FrameRate parses a frame rate such as "24000/1001"; it reports false for
// a malformed value or a zero denominator.
func FrameRate(value string) (float32, bool) {
	r := parseRational(value)
	if r.Den == 0 {
		return 0, false
	}
	return r.Float32(), true
}

func parseRational(value string) core.Rational {
	num, den, ok := strings.Cut(value, "/")
	if !ok {
		return core.Rational{}
	}
	n, err1 := strconv.ParseInt(num, 10, 64)
	d, err2 := strconv.ParseInt(den, 10, 64)
	if err1 != nil || err2 != nil {
		return core.Rational{}
	}
	return core.Rational{Num: n, Den: d}
}

// parseRatio parses "num:den"; square or unknown pixels are zero.
func parseRatio(value string) core.Rational {
	r := parseRational(strings.Replace(value, ":", "/", 1))
	if r.Num == r.Den || r.Num == 0 || r.Den == 0 {
		return core.Rational{}
	}
	return r
}

func bpsTag(s *Stream) int {
	v, _ := s.Tags.get("BPS-eng", "BPS")
	n, _ := strconv.Atoi(v)
	return n
}

func durationTag(s *Stream) (float64, bool) {
	v, _ := s.Tags.get("DURATION-eng", "DURATION")
	if v == "" {
		return 0, false
	}
	// Matroska durations have nanoseconds; .NET TimeSpan parsing, which
	// Jellyfin uses, accepts seven fractional digits.
	v = durationOverPrecison.ReplaceAllString(v, "$1")
	return parseTimeSpan(v)
}

// parseTimeSpan parses [d.]hh:mm:ss[.fffffff] into seconds.
func parseTimeSpan(v string) (float64, bool) {
	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		return 0, false
	}
	var days float64
	hours := parts[0]
	if d, h, ok := strings.Cut(hours, "."); ok {
		dd, err := strconv.ParseFloat(d, 64)
		if err != nil {
			return 0, false
		}
		days, hours = dd, h
	}
	h, err1 := strconv.ParseFloat(hours, 64)
	m, err2 := strconv.ParseFloat(parts[1], 64)
	sec, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return days*86400 + h*3600 + m*60 + sec, true
}

func bytesTag(s *Stream) (int64, bool) {
	v, _ := s.Tags.get("NUMBER_OF_BYTES-eng", "NUMBER_OF_BYTES")
	n, err := strconv.ParseInt(v, 10, 64)
	return n, err == nil
}

func numericTag(tags Tags, key string) *int {
	v, ok := tags.get(key)
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return nil
	}
	return &n
}

var tagDateLayouts = []string{
	time.RFC3339Nano, "2006-01-02T15:04:05", time.DateTime, time.DateOnly, "2006/01/02", "2006-01", "2006",
	"01/02/2006", "01/02/2006 15:04:05",
}

// dateTag parses a date tag as UTC.
func dateTag(tags Tags, key string) (time.Time, bool) {
	v, ok := tags.get(key)
	if !ok {
		return time.Time{}, false
	}
	v = strings.TrimSpace(v)
	for _, l := range tagDateLayouts {
		if t, err := time.Parse(l, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func chapter(c Chapter) core.Chapter {
	var ch core.Chapter
	ch.Title = c.Tags["title"]
	// Millisecond precision, as chapters are saved.
	if secs, err := strconv.ParseFloat(c.StartTime, 64); err == nil {
		ch.Start = time.Duration(math.RoundToEven(secs*1000)) * time.Millisecond
	}
	return ch
}

func audioRuntime(out *Output, r *Result) {
	for _, s := range out.Streams {
		if s.CodecType != "audio" {
			continue
		}
		d := s.Duration
		if d == "" && out.Format != nil {
			d = out.Format.Duration
		}
		if secs, err := strconv.ParseFloat(d, 64); err == nil {
			r.Source.Duration = seconds(secs)
			r.Metadata.Runtime = r.Source.Duration
		}
		return
	}
}

// audioTags reads the credits, album artists, numbering, studios and
// MusicBrainz IDs of an audio file.
func audioTags(r *Result, tags Tags) {
	md := &r.Metadata
	add := func(key string, kind core.CreditKind) {
		if v, ok := tags.get(key); ok && strings.TrimSpace(v) != "" {
			for _, name := range split(v, false) {
				r.People = append(r.People, Person{Name: name, Kind: kind})
			}
		}
	}
	add("composer", core.CreditComposer)
	add("conductor", core.CreditConductor)
	add("lyricist", core.CreditLyricist)
	if v, ok := tags.get("performer"); ok && strings.TrimSpace(v) != "" {
		for _, p := range split(v, false) {
			// A performer without an instrument is most likely a band.
			if m := performerRegex.FindStringSubmatch(p); m != nil {
				r.People = append(r.People, Person{Name: m[1], Kind: core.CreditActor, Role: titleCase(m[2])})
			}
		}
	}
	// "writer" is used when the role on the recording is unknown.
	add("writer", core.CreditWriter)
	add("arranger", core.CreditOther)
	add("engineer", core.CreditOther)
	add("mixer", core.CreditOther)
	add("remixer", core.CreditOther)

	if a := tags.firstNonBlank("albumartist", "album artist", "album_artist"); a != "" {
		md.AlbumArtists = splitDistinctArtists(a, nameDelimiters, true)
	}
	if len(md.AlbumArtists) == 0 {
		md.AlbumArtists = md.Artists
	}
	md.IndexNumber = trackOrDisc(tags, "track")
	md.ParentIndexNumber = trackOrDisc(tags, "disc")
	for _, key := range []string{"organization", "ensemble", "publisher", "label"} {
		fetchStudios(md, tags, key)
	}
	for _, id := range []struct {
		provider core.Provider
		keys     []string
	}{
		{core.ProviderMusicBrainzAlbumArtist, []string{"MusicBrainz Album Artist Id", "MUSICBRAINZ_ALBUMARTISTID"}},
		{core.ProviderMusicBrainzArtist, []string{"MusicBrainz Artist Id", "MUSICBRAINZ_ARTISTID"}},
		{core.ProviderMusicBrainzAlbum, []string{"MusicBrainz Album Id", "MUSICBRAINZ_ALBUMID"}},
		{core.ProviderMusicBrainzReleaseGroup, []string{"MusicBrainz Release Group Id", "MUSICBRAINZ_RELEASEGROUPID"}},
		{core.ProviderMusicBrainzTrack, []string{"MusicBrainz Release Track Id", "MUSICBRAINZ_RELEASETRACKID"}},
	} {
		for _, k := range id.keys {
			// Several IDs may be listed; only the first is kept.
			if parts := splitNonEmpty(tags[strings.ToLower(k)], "/"); len(parts) > 0 {
				if md.ExternalIDs == nil {
					md.ExternalIDs = map[core.Provider]string{}
				}
				md.ExternalIDs[id.provider] = parts[0]
				break
			}
		}
	}
}

func trackOrDisc(tags Tags, key string) *int {
	v := tags[key]
	left, _, _ := strings.Cut(v, "/")
	n, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil {
		return nil
	}
	return &n
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(strings.ToLower(w))
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// split splits a list of names on "/", ";", "|" or "\", or on commas when
// allowed and none of those occur.
func split(v string, allowComma bool) []string {
	if !allowComma || strings.ContainsAny(v, nameDelimiters) {
		return splitAny(v, nameDelimiters)
	}
	return splitAny(v, ",")
}

func splitAny(v, delimiters string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(v, func(r rune) bool { return strings.ContainsRune(delimiters, r) }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitNonEmpty(v, sep string) []string { return splitAny(v, sep) }

// splitDistinctArtists splits artist names, keeping names that contain
// delimiters ("AC/DC") whole.
func splitDistinctArtists(v, delimiters string, splitFeaturing bool) []string {
	if splitFeaturing {
		v = replaceFold(replaceFold(v, " featuring ", " | "), " feat. ", " | ")
	}
	var found []string
	for _, w := range splitWhitelist {
		if replaced := replaceFold(v, w, "|"); !strings.EqualFold(replaced, v) {
			found = append(found, w)
			v = replaced
		}
	}
	found = append(found, splitAny(v, delimiters)...)
	return distinctFold(found)
}

func distinctFold(list []string) []string {
	var out []string
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s != "" && !containsFold(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func fetchStudios(md *core.Item, tags Tags, key string) {
	v := tags[key]
	if v == "" {
		return
	}
	var studios []string
	for _, s := range split(v, true) {
		// Artists listed as studios are not studios.
		if containsFold(md.Artists, s) || containsFold(md.AlbumArtists, s) {
			continue
		}
		studios = append(studios, s)
	}
	md.Studios = distinctFold(studios)
}

func genres(existing []string, tags Tags) []string {
	v := tags["genre"]
	if v == "" {
		return existing
	}
	return distinctFold(append(slices.Clone(existing), split(v, true)...))
}

// wtvInfo reads Windows Media Center recording tags.
func wtvInfo(r *Result, out *Output) {
	if out.Format == nil || out.Format.Tags == nil {
		return
	}
	tags := out.Format.Tags
	md := &r.Metadata
	if v := tags["wm/genre"]; strings.TrimSpace(v) != "" {
		if list := splitAny(v, genreDelimiters); len(list) > 0 {
			md.Genres = list
		}
	}
	if v := tags["wm/parentalrating"]; strings.TrimSpace(v) != "" {
		md.OfficialRating = v
	}
	if v := tags["wm/mediacredits"]; v != "" {
		r.People = nil
		for _, name := range splitAny(v, basicDelimiters) {
			r.People = append(r.People, Person{Name: name, Kind: core.CreditActor})
		}
	}
	if y, err := strconv.Atoi(tags["wm/originalreleasetime"]); err == nil {
		md.ProductionYear = y
	}
	description, subtitle := tags["wm/subtitledescription"], tags["wm/subtitle"]
	// When the subtitle is empty the description may start with it:
	// "4/13. The Doctor's Wife: Science fiction drama. …" or
	// "Comment. Subtitle: description".
	if strings.TrimSpace(subtitle) == "" && strings.TrimSpace(description) != "" &&
		strings.Contains(description[:min(len(description), 100)], ":") {
		first, _, _ := strings.Cut(description, ":")
		switch {
		case strings.Contains(first, "/"):
			parts := strings.Split(first, " ")
			numbers := strings.Split(strings.ReplaceAll(parts[0], ".", ""), "/")
			if n, err := strconv.Atoi(numbers[0]); err == nil {
				md.IndexNumber = &n
				description = strings.TrimSpace(strings.Join(parts[1:], " "))
			} else if strings.Contains(first, ".") {
				description = strings.TrimSpace(strings.Join(strings.Split(first, ".")[1:], "."))
			} else {
				description = strings.TrimSpace(first)
			}
		case strings.Contains(first, "."):
			description = strings.TrimSpace(strings.Join(strings.Split(first, ".")[1:], "."))
		default:
			description = strings.TrimSpace(first)
		}
	}
	if strings.TrimSpace(description) != "" {
		md.Overview = description
	}
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(v string) bool { return strings.EqualFold(v, s) })
}

// replaceFold replaces every case-insensitive occurrence of old in s.
func replaceFold(s, old, repl string) string {
	if old == "" {
		return s
	}
	lower, lowerOld := strings.ToLower(s), strings.ToLower(old)
	if len(lower) != len(s) {
		return s // case mapping changed the length; leave as is
	}
	var b strings.Builder
	for {
		i := strings.Index(lower, lowerOld)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(repl)
		s, lower = s[i+len(old):], lower[i+len(old):]
	}
}
