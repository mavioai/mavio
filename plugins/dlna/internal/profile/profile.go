// Package profile describes what DLNA devices play: each profile holds a
// device's capabilities as the server's playback decisions take them, its
// DLNA quirks, and the rules that recognize the device by its description
// or its requests' headers.
package profile

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"

	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
)

// GenericName names the profile of devices no other profile matches.
const GenericName = "Generic"

// Profile is a device profile.
type Profile struct {
	Name string `json:"name"`
	// Match lists the rules recognizing the device; any of them may match.
	Match []Rule `json:"match,omitempty"`
	// Capabilities is a mavio.playback.v1.ClientCapabilities in JSON.
	Capabilities json.RawMessage `json:"capabilities"`
	// TimeSeek is set for devices seeking transcodes by time, with
	// TimeSeekRange.dlna.org.
	TimeSeek bool `json:"time_seek,omitempty"`
	// CaptionInfo is set for devices that read subtitles from Samsung's
	// sec:CaptionInfoEx and CaptionInfo.sec header.
	CaptionInfo bool `json:"caption_info,omitempty"`
	// SubtitleRes is set for devices that read subtitles from a res of
	// the item.
	SubtitleRes bool `json:"subtitle_res,omitempty"`
	// MimeTypes replaces the MIME types of containers, such as
	// {"mkv": "video/x-mkv"}.
	MimeTypes map[string]string `json:"mime_types,omitempty"`

	caps  *playbackv1.ClientCapabilities
	rules []rule
}

// Rule recognizes a device: every field set must match, as a regular
// expression ignoring case.
type Rule struct {
	FriendlyName string `json:"friendly_name,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ModelNumber  string `json:"model_number,omitempty"`
	// Headers match request headers by name, such as User-Agent.
	Headers map[string]string `json:"headers,omitempty"`
}

type rule struct {
	fields  [4]*regexp.Regexp
	headers map[string]*regexp.Regexp
}

// Device is what is known of a device: its description, when it has been
// read, and the headers of its requests.
type Device struct {
	FriendlyName, Manufacturer, ModelName, ModelNumber string
	Header                                             http.Header
}

//go:embed profiles.json
var builtin []byte

// Set is the profiles in the order they are tried, the generic one last.
type Set struct{ profiles []*Profile }

// NewSet returns the built-in profiles with overrides: an override
// replaces the built-in profile of its name, and is tried before the
// built-in ones otherwise.
func NewSet(overrides []Profile) (*Set, error) {
	var base []Profile
	if err := json.Unmarshal(builtin, &base); err != nil {
		return nil, fmt.Errorf("built-in profiles: %w", err)
	}
	var out []*Profile
	for i := range overrides {
		p := overrides[i]
		if err := p.compile(); err != nil {
			return nil, fmt.Errorf("profile %q: %w", p.Name, err)
		}
		if slices.ContainsFunc(out, func(q *Profile) bool { return q.Name == p.Name }) {
			return nil, fmt.Errorf("profile %q is given twice", p.Name)
		}
		out = append(out, &p)
	}
	for i := range base {
		p := base[i]
		if slices.ContainsFunc(out, func(q *Profile) bool { return q.Name == p.Name }) {
			continue
		}
		if err := p.compile(); err != nil {
			return nil, fmt.Errorf("built-in profile %q: %w", p.Name, err)
		}
		out = append(out, &p)
	}
	// The generic profile goes last, whichever its source.
	i := slices.IndexFunc(out, func(p *Profile) bool { return p.Name == GenericName })
	if i < 0 {
		return nil, errors.New("no generic profile")
	}
	g := out[i]
	out = append(slices.Delete(out, i, i+1), g)
	return &Set{profiles: out}, nil
}

func (p *Profile) compile() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("a profile needs a name")
	}
	p.caps = &playbackv1.ClientCapabilities{}
	if len(p.Capabilities) > 0 {
		if err := protojson.Unmarshal(p.Capabilities, p.caps); err != nil {
			return fmt.Errorf("capabilities: %w", err)
		}
	}
	if p.caps.GetName() == "" {
		p.caps.SetName("DLNA " + p.Name)
	}
	if err := protovalidate.Validate(p.caps); err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	for _, r := range p.Match {
		var c rule
		for i, expr := range []string{r.FriendlyName, r.Manufacturer, r.ModelName, r.ModelNumber} {
			if expr == "" {
				continue
			}
			re, err := regexp.Compile("(?i)" + expr)
			if err != nil {
				return err
			}
			c.fields[i] = re
		}
		for h, expr := range r.Headers {
			re, err := regexp.Compile("(?i)" + expr)
			if err != nil {
				return err
			}
			if c.headers == nil {
				c.headers = map[string]*regexp.Regexp{}
			}
			c.headers[http.CanonicalHeaderKey(h)] = re
		}
		if c.headers == nil && c.fields == [4]*regexp.Regexp{} {
			return errors.New("a rule needs a field to match")
		}
		p.rules = append(p.rules, c)
	}
	return nil
}

// ClientCapabilities returns the profile's capabilities; callers must
// not change them.
func (p *Profile) ClientCapabilities() *playbackv1.ClientCapabilities { return p.caps }

// MimeType returns the MIME type of media in a container, of a kind:
// "video", "audio" or "image".
func (p *Profile) MimeType(container, kind string) string {
	for c := range strings.SplitSeq(strings.ToLower(container), ",") {
		if t, ok := p.MimeTypes[c]; ok {
			return t
		}
	}
	return DefaultMimeType(container, kind)
}

// Match returns the first profile matching a device, else the generic
// one.
func (s *Set) Match(d Device) *Profile {
	for _, p := range s.profiles {
		if p.matches(&d) {
			return p
		}
	}
	return s.profiles[len(s.profiles)-1]
}

// Get returns a profile by name.
func (s *Set) Get(name string) (*Profile, bool) {
	i := slices.IndexFunc(s.profiles, func(p *Profile) bool { return p.Name == name })
	if i < 0 {
		return nil, false
	}
	return s.profiles[i], true
}

// Generic returns the generic profile.
func (s *Set) Generic() *Profile { return s.profiles[len(s.profiles)-1] }

func (p *Profile) matches(d *Device) bool {
	values := [4]string{d.FriendlyName, d.Manufacturer, d.ModelName, d.ModelNumber}
	for _, r := range p.rules {
		ok := true
		for i, re := range r.fields {
			if re != nil && (values[i] == "" || !re.MatchString(values[i])) {
				ok = false
			}
		}
		for h, re := range r.headers {
			if v := d.Header.Get(h); v == "" || !re.MatchString(v) {
				ok = false
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// mimeTypes are the usual MIME types of containers, by kind.
var mimeTypes = map[string]map[string]string{
	"video": {
		"mp4": "video/mp4", "m4v": "video/mp4", "mov": "video/quicktime", "mkv": "video/x-matroska", "webm": "video/webm",
		"ts": "video/mp2t", "m2ts": "video/mp2t", "mpeg": "video/mpeg", "mpg": "video/mpeg", "avi": "video/x-msvideo",
		"asf": "video/x-ms-asf", "wmv": "video/x-ms-wmv", "flv": "video/x-flv", "3gp": "video/3gpp", "ogg": "video/ogg",
	},
	"audio": {
		"mp3": "audio/mpeg", "flac": "audio/flac", "mp4": "audio/mp4", "m4a": "audio/mp4", "aac": "audio/aac", "adts": "audio/aac",
		"ogg": "audio/ogg", "oga": "audio/ogg", "opus": "audio/ogg", "wav": "audio/wav", "asf": "audio/x-ms-wma", "wma": "audio/x-ms-wma",
		"aiff": "audio/aiff", "ape": "audio/x-ape", "wv": "audio/x-wavpack", "mka": "audio/x-matroska", "mkv": "audio/x-matroska",
	},
	"image": {"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "gif": "image/gif", "webp": "image/webp"},
}

// DefaultMimeType returns the usual MIME type of media in a container,
// which may be a list as ffprobe names formats ("mov,mp4,m4a,3gp").
func DefaultMimeType(container, kind string) string {
	names := strings.Split(strings.ToLower(container), ",")
	// ffprobe's list for MP4 starts with "mov"; prefer the MP4 type.
	if slices.Contains(names, "mp4") {
		names = append([]string{"mp4"}, names...)
	}
	for _, c := range names {
		if t, ok := mimeTypes[kind][c]; ok {
			return t
		}
	}
	return map[string]string{"video": "video/mpeg", "audio": "audio/mpeg", "image": "image/jpeg"}[kind]
}

// ContainsName reports whether a list of names, as client capabilities
// write them, admits any of the names given: an empty list admits all, a
// list starting with "-" all but those it names.
func ContainsName(list, names string) bool {
	list = strings.TrimSpace(list)
	if list == "" {
		return true
	}
	negate := strings.HasPrefix(list, "-")
	var listed []string
	for n := range strings.SplitSeq(strings.ToLower(strings.TrimPrefix(list, "-")), ",") {
		listed = append(listed, strings.TrimSpace(n))
	}
	found := false
	for n := range strings.SplitSeq(strings.ToLower(names), ",") {
		if n != "" && slices.Contains(listed, strings.TrimSpace(n)) {
			found = true
		}
	}
	return found != negate
}
