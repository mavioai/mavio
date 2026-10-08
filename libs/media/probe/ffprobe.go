package probe

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Output is ffprobe's JSON output (-print_format json -show_streams
// -show_format -show_chapters, optionally -show_frames).
type Output struct {
	Streams  []Stream  `json:"streams"`
	Format   *Format   `json:"format"`
	Chapters []Chapter `json:"chapters"`
	Frames   []Frame   `json:"frames"`
}

// Format is the container section of ffprobe's output.
type Format struct {
	FileName   string `json:"filename"`
	FormatName string `json:"format_name"`
	StartTime  string `json:"start_time"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
	Tags       Tags   `json:"tags"`
}

// Stream is a stream section of ffprobe's output.
type Stream struct {
	Index              int            `json:"index"`
	Profile            string         `json:"profile"`
	CodecName          string         `json:"codec_name"`
	CodecType          string         `json:"codec_type"`
	SampleRate         string         `json:"sample_rate"`
	Channels           *Int           `json:"channels"`
	ChannelLayout      string         `json:"channel_layout"`
	AverageFrameRate   string         `json:"avg_frame_rate"`
	Duration           string         `json:"duration"`
	BitRate            string         `json:"bit_rate"`
	Width              *Int           `json:"width"`
	Height             *Int           `json:"height"`
	Refs               Int            `json:"refs"`
	DisplayAspectRatio string         `json:"display_aspect_ratio"`
	Tags               Tags           `json:"tags"`
	BitsPerSample      Int            `json:"bits_per_sample"`
	BitsPerRawSample   Int            `json:"bits_per_raw_sample"`
	RealFrameRate      string         `json:"r_frame_rate"`
	SampleAspectRatio  string         `json:"sample_aspect_ratio"`
	PixelFormat        string         `json:"pix_fmt"`
	Level              *Int           `json:"level"`
	TimeBase           string         `json:"time_base"`
	CodecTimeBase      string         `json:"codec_time_base"`
	CodecTagString     string         `json:"codec_tag_string"`
	IsAVC              Bool           `json:"is_avc"`
	NALLengthSize      String         `json:"nal_length_size"`
	FieldOrder         string         `json:"field_order"`
	Disposition        map[string]Int `json:"disposition"`
	ColorRange         string         `json:"color_range"`
	ColorSpace         string         `json:"color_space"`
	ColorTransfer      string         `json:"color_transfer"`
	ColorPrimaries     string         `json:"color_primaries"`
	SideData           []SideData     `json:"side_data_list"`
}

// SideData is a stream or frame side data entry.
type SideData struct {
	Type                      string `json:"side_data_type"`
	DVVersionMajor            Int    `json:"dv_version_major"`
	DVVersionMinor            Int    `json:"dv_version_minor"`
	DVProfile                 Int    `json:"dv_profile"`
	DVLevel                   Int    `json:"dv_level"`
	RPUPresentFlag            Int    `json:"rpu_present_flag"`
	ELPresentFlag             Int    `json:"el_present_flag"`
	BLPresentFlag             Int    `json:"bl_present_flag"`
	DVBLSignalCompatibilityID Int    `json:"dv_bl_signal_compatibility_id"`
	Rotation                  Int    `json:"rotation"`
}

// Chapter is a chapter section of ffprobe's output.
type Chapter struct {
	StartTime string `json:"start_time"`
	Tags      Tags   `json:"tags"`
}

// Frame is a frame section of ffprobe's output.
type Frame struct {
	StreamIndex int        `json:"stream_index"`
	SideData    []SideData `json:"side_data_list"`
}

// Tags are metadata tags; keys are matched case-insensitively.
type Tags map[string]string

// UnmarshalJSON lower-cases the keys, keeping the first of keys that
// differ only in case, and accepts non-string values.
func (t *Tags) UnmarshalJSON(data []byte) error {
	out := Tags{}
	d := json.NewDecoder(bytes.NewReader(data))
	if _, err := d.Token(); err != nil { // {
		return err
	}
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return err
		}
		lk := strings.ToLower(key)
		if _, dup := out[lk]; dup {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			s = string(v)
		}
		out[lk] = s
	}
	*t = out
	return nil
}

// get returns the first of the keys that is set.
func (t Tags) get(keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := t[strings.ToLower(k)]; ok {
			return v, true
		}
	}
	return "", false
}

// firstNonBlank returns the first value of the keys that is not blank.
func (t Tags) firstNonBlank(keys ...string) string {
	for _, k := range keys {
		if v := t[strings.ToLower(k)]; strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Int is an integer that ffprobe may print as a JSON number or string.
type Int int

// UnmarshalJSON accepts a number or a numeric string.
func (i *Int) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" || s == "N/A" {
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // tolerate garbage, as Jellyfin does
	}
	*i = Int(n)
	return nil
}

// Bool is a boolean that ffprobe may print as a string ("true", "1").
type Bool bool

// UnmarshalJSON accepts a boolean, a number or a string.
func (b *Bool) UnmarshalJSON(data []byte) error {
	s := strings.ToLower(strings.Trim(string(data), `"`))
	*b = s == "true" || s == "1"
	return nil
}

// String is a string that ffprobe may print as a number.
type String string

// UnmarshalJSON accepts a string or a number.
func (s *String) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	*s = String(strings.Trim(string(data), `"`))
	return nil
}
