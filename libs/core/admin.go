package core

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
)

// ServerSettings are what administrators change while the server runs;
// command-line flags only give the first values of some.
type ServerSettings struct {
	Transcoding TranscodingSettings
	Network     NetworkSettings
	// PluginCatalogs are the http, https or file URLs of the plugin
	// catalogs plugins are installed from.
	PluginCatalogs []string
	// PasswordResetPlugin is the ID of the plugin that delivers the PINs
	// resetting forgotten passwords; empty when users cannot reset them.
	PasswordResetPlugin string
	// DisabledPlugins are the IDs of the installed plugins administrators
	// disabled, which the server does not start.
	DisabledPlugins []string
	// SessionIdleDays is how long a session lasts unused, 1–3650 days.
	SessionIdleDays int
	Acquisition     AcquisitionSettings
	// Requests are the defaults and destinations of users' requests.
	Requests RequestSettings
	// Metadata are the language and country of metadata beside
	// libraries'.
	Metadata MetadataSettings
	// Subtitles are how subtitles are acquired.
	Subtitles SubtitleSettings
	UpdatedAt time.Time
}

// MetadataSettings are the server's language and country of metadata
// beside libraries', as Jellyfin's PreferredMetadataLanguage and
// MetadataCountryCode: requests and discovery name titles in the language
// and rate them in the country.
type MetadataSettings struct {
	// PreferredLanguage is ISO 639-1, such as "en".
	PreferredLanguage string
	// Country is ISO 3166-1, such as "US".
	Country string
}

// Validate checks the settings' codes.
func (m *MetadataSettings) Validate() error {
	notLower := func(r rune) bool { return r < 'a' || r > 'z' }
	notUpper := func(r rune) bool { return r < 'A' || r > 'Z' }
	switch {
	case m.PreferredLanguage != "" && (len(m.PreferredLanguage) != 2 || strings.ContainsFunc(m.PreferredLanguage, notLower)):
		return fmt.Errorf("%w: metadata language %q is not ISO 639-1", ErrInvalid, m.PreferredLanguage)
	case m.Country != "" && (len(m.Country) != 2 || strings.ContainsFunc(m.Country, notUpper)):
		return fmt.Errorf("%w: metadata country %q is not ISO 3166-1", ErrInvalid, m.Country)
	}
	return nil
}

// HardwareAcceleration chooses how video is decoded and encoded.
type HardwareAcceleration string

// Hardware accelerations.
const (
	// HardwareAuto uses the acceleration the server's ffmpeg and hardware
	// support, if any.
	HardwareAuto         HardwareAcceleration = "auto"
	HardwareNone         HardwareAcceleration = "none"
	HardwareVideoToolbox HardwareAcceleration = "videotoolbox"
)

// HardwareAccelerations lists the known accelerations.
var HardwareAccelerations = []HardwareAcceleration{HardwareAuto, HardwareNone, HardwareVideoToolbox}

// TranscodingSettings steer transcodes.
type TranscodingSettings struct {
	HardwareAcceleration HardwareAcceleration
	// HardwareEncoding allows hardware encoders where decoding is
	// accelerated.
	HardwareEncoding bool
	// EncoderPreset is the software encoders' speed preset, such as
	// "veryfast"; empty picks one by the source.
	EncoderPreset string
	// H264CRF and H265CRF are the software encoders' quality, 0–51.
	H264CRF, H265CRF int
	// Threads is the encoder thread count; zero lets ffmpeg decide.
	Threads int
	// TonemapAlgorithm, TonemapRange, TonemapDesat and TonemapPeak convert
	// HDR to SDR.
	TonemapAlgorithm string
	TonemapRange     string
	TonemapDesat     float64
	TonemapPeak      float64
	// DeinterlaceMethod is "yadif" or "bwdif"; DeinterlaceDoubleRate keeps
	// every field as a frame.
	DeinterlaceMethod     string
	DeinterlaceDoubleRate bool
	// DownmixBoost amplifies audio downmixed to stereo; 1 leaves it.
	DownmixBoost float64
	// CropBlackBorders crops the black borders found in videos.
	CropBlackBorders bool
	// TranscodeDir holds running transcodes; empty uses the folder the
	// server was started with.
	TranscodeDir string
}

// Known values of transcoding settings.
var (
	EncoderPresets     = []string{"", "ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}
	TonemapAlgorithms  = []string{"bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma"}
	TonemapRanges      = []string{"auto", "tv", "pc"}
	DeinterlaceMethods = []string{"yadif", "bwdif"}
)

// NetworkSettings decide how the server is reached.
type NetworkSettings struct {
	// ServerName names the server to clients; empty uses the host name.
	ServerName string
	// BaseURL is the path a reverse proxy serves the server under, such as
	// "/mavio"; empty serves it at the root.
	BaseURL string
	// HTTPSPort, when set, serves HTTPS on that port with the certificate
	// and key in CertificatePath and KeyPath (PEM files) or, without them,
	// in mavio.dev mode with the server's *.<ID>.mavio.site certificate.
	HTTPSPort                int
	CertificatePath, KeyPath string
	// LocalDiscovery answers discovery requests from clients on the local
	// network.
	LocalDiscovery bool
	// Relay, in mavio.dev mode, lets clients reach the server through
	// mavio.dev's relay when they cannot reach it directly.
	Relay bool
}

// DefaultServerSettings are the settings of a new server.
func DefaultServerSettings() ServerSettings {
	return ServerSettings{
		Transcoding: TranscodingSettings{
			HardwareAcceleration: HardwareAuto,
			HardwareEncoding:     true,
			H264CRF:              23,
			H265CRF:              28,
			TonemapAlgorithm:     "bt2390",
			TonemapRange:         "auto",
			TonemapPeak:          100,
			DeinterlaceMethod:    "yadif",
			DownmixBoost:         2,
			CropBlackBorders:     true,
		},
		Network:         NetworkSettings{HTTPSPort: 8920, LocalDiscovery: true, Relay: true},
		SessionIdleDays: 90,
		Acquisition:     DefaultAcquisitionSettings(),
		Requests:        DefaultRequestSettings(),
		Metadata:        MetadataSettings{PreferredLanguage: "en", Country: "US"},
		Subtitles:       DefaultSubtitleSettings(),
	}
}

// Validate checks the settings' invariants.
func (s *ServerSettings) Validate() error {
	t, n := &s.Transcoding, &s.Network
	switch {
	case !slices.Contains(HardwareAccelerations, t.HardwareAcceleration):
		return fmt.Errorf("%w: unknown hardware acceleration %q", ErrInvalid, t.HardwareAcceleration)
	case !slices.Contains(EncoderPresets, t.EncoderPreset):
		return fmt.Errorf("%w: unknown encoder preset %q", ErrInvalid, t.EncoderPreset)
	case t.H264CRF < 0 || t.H264CRF > 51 || t.H265CRF < 0 || t.H265CRF > 51:
		return fmt.Errorf("%w: CRF outside 0–51", ErrInvalid)
	case t.Threads < 0:
		return fmt.Errorf("%w: negative thread count", ErrInvalid)
	case !slices.Contains(TonemapAlgorithms, t.TonemapAlgorithm):
		return fmt.Errorf("%w: unknown tone mapping algorithm %q", ErrInvalid, t.TonemapAlgorithm)
	case !slices.Contains(TonemapRanges, t.TonemapRange):
		return fmt.Errorf("%w: unknown tone mapping range %q", ErrInvalid, t.TonemapRange)
	case t.TonemapDesat < 0 || t.TonemapPeak < 0:
		return fmt.Errorf("%w: negative tone mapping parameter", ErrInvalid)
	case !slices.Contains(DeinterlaceMethods, t.DeinterlaceMethod):
		return fmt.Errorf("%w: unknown deinterlacing method %q", ErrInvalid, t.DeinterlaceMethod)
	case t.DownmixBoost < 0.5 || t.DownmixBoost > 3:
		return fmt.Errorf("%w: downmix boost outside 0.5–3", ErrInvalid)
	case t.TranscodeDir != "" && !isAbs(t.TranscodeDir):
		return fmt.Errorf("%w: transcode folder %q is not absolute", ErrInvalid, t.TranscodeDir)
	case n.BaseURL != "" && !validBaseURL(n.BaseURL):
		return fmt.Errorf("%w: base URL %q is not a path such as /mavio", ErrInvalid, n.BaseURL)
	case n.HTTPSPort < 0 || n.HTTPSPort > 65535:
		return fmt.Errorf("%w: HTTPS port %d", ErrInvalid, n.HTTPSPort)
	case (n.CertificatePath == "") != (n.KeyPath == ""):
		return fmt.Errorf("%w: HTTPS needs both a certificate and a key", ErrInvalid)
	case s.SessionIdleDays < 1 || s.SessionIdleDays > 3650:
		return fmt.Errorf("%w: session idle days %d outside 1–3650", ErrInvalid, s.SessionIdleDays)
	}
	if err := s.Acquisition.Validate(); err != nil {
		return err
	}
	if err := s.Requests.Validate(); err != nil {
		return err
	}
	if err := s.Metadata.Validate(); err != nil {
		return err
	}
	if err := s.Subtitles.Validate(); err != nil {
		return err
	}
	for _, c := range s.PluginCatalogs {
		if !validCatalog(c) {
			return fmt.Errorf("%w: plugin catalog %q is not an http, https or file URL", ErrInvalid, c)
		}
	}
	return nil
}

// validCatalog reports whether s is the http or https URL of a plugin
// catalog, or the file URL of one in a checkout of the repository.
func validCatalog(s string) bool {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return false
	case u.Scheme == "http" || u.Scheme == "https":
		return u.Host != ""
	case u.Scheme == "file":
		return u.Host == "" && strings.HasPrefix(u.Path, "/")
	}
	return false
}

// validBaseURL reports whether s is a clean absolute path of unreserved
// URL characters, such as "/mavio".
func validBaseURL(s string) bool {
	if !strings.HasPrefix(s, "/") || s == "/" || path.Clean(s) != s {
		return false
	}
	return strings.Trim(s, "/abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~") == ""
}

// isAbs reports whether p is an absolute path, in Unix or Windows form.
func isAbs(p string) bool {
	return strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\\`) ||
		len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

// APIKey lets an integration call the API as the administrator who
// created it, with a token of its own.
type APIKey struct {
	ID     ID
	UserID ID
	// Name says what the key is for, such as "Home Assistant".
	Name string
	// TokenHash is the SHA-256 hash of the token, which is shown once.
	TokenHash  []byte
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// Validate checks the key's invariants.
func (k *APIKey) Validate() error {
	switch {
	case k.ID.IsZero() || k.UserID.IsZero():
		return fmt.Errorf("%w: API key requires ID and user", ErrInvalid)
	case strings.TrimSpace(k.Name) == "":
		return fmt.Errorf("%w: API key %s requires a name", ErrInvalid, k.ID)
	case len(k.TokenHash) != 32:
		return fmt.Errorf("%w: API key %s requires a SHA-256 token hash", ErrInvalid, k.ID)
	}
	return nil
}

// Severity grades an activity.
type Severity string

// Severities, from the least severe.
const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Valid reports whether s is a known severity.
func (s Severity) Valid() bool {
	return s == SeverityInfo || s == SeverityWarning || s == SeverityError
}

// Activity is something that happened on the server, kept in the activity
// log and sent to notification plugins.
type Activity struct {
	ID   ID
	Time time.Time
	// Type is dotted, such as "user.login_failed"; see docs/domain.md.
	Type     string
	Severity Severity
	// Title is a short summary, Message details.
	Title, Message string
	// UserID and ItemID name the user and item it concerns, if any.
	UserID, ItemID ID
	// Attributes are type-specific details.
	Attributes map[string]string
}

// Validate checks the activity's invariants.
func (a *Activity) Validate() error {
	switch {
	case a.ID.IsZero() || a.Type == "" || a.Time.IsZero():
		return fmt.Errorf("%w: activity requires ID, type and time", ErrInvalid)
	case !a.Severity.Valid():
		return fmt.Errorf("%w: unknown severity %q", ErrInvalid, a.Severity)
	case a.Title == "":
		return fmt.Errorf("%w: activity %s requires a title", ErrInvalid, a.ID)
	}
	return nil
}

// ActivityQuery selects activities, newest first; unset fields do not
// filter.
type ActivityQuery struct {
	Since  time.Time
	UserID ID
	// MinSeverity leaves out less severe activities.
	MinSeverity Severity
	// Types keeps activities of these types only.
	Types  []string
	Limit  int // zero means MaxPageSize
	Offset int
}

// JobQuery selects jobs, most recently created first; unset fields do
// not filter.
type JobQuery struct {
	Kinds  []string
	States []JobState
	Limit  int // zero means MaxPageSize
	Offset int
}

// Validate checks the query's bounds.
func (q *ActivityQuery) Validate() error {
	switch {
	case q.Limit < 0 || q.Limit > MaxPageSize || q.Offset < 0:
		return fmt.Errorf("%w: page outside 0–%d", ErrInvalid, MaxPageSize)
	case q.MinSeverity != "" && !q.MinSeverity.Valid():
		return fmt.Errorf("%w: unknown severity %q", ErrInvalid, q.MinSeverity)
	}
	return nil
}

// PageSize is the effective limit.
func (q *ActivityQuery) PageSize() int {
	if q.Limit == 0 {
		return MaxPageSize
	}
	return q.Limit
}

// Validate checks the query's bounds.
func (q *JobQuery) Validate() error {
	if q.Limit < 0 || q.Limit > MaxPageSize || q.Offset < 0 {
		return fmt.Errorf("%w: page outside 0–%d", ErrInvalid, MaxPageSize)
	}
	return nil
}

// PageSize is the effective limit.
func (q *JobQuery) PageSize() int {
	if q.Limit == 0 {
		return MaxPageSize
	}
	return q.Limit
}

// AtLeast reports whether s is at least as severe as min.
func (s Severity) AtLeast(min Severity) bool {
	rank := map[Severity]int{SeverityInfo: 0, SeverityWarning: 1, SeverityError: 2}
	return rank[s] >= rank[min]
}

// AccountMode is how a server signs people in, chosen on first start.
type AccountMode string

// Account modes.
const (
	// AccountModeUnset is a server whose setup has not chosen a mode.
	AccountModeUnset AccountMode = ""
	// AccountModeStandalone servers keep their own users.
	AccountModeStandalone AccountMode = "standalone"
	// AccountModeMavioDev servers sign in Mavio accounts kept by
	// mavio.dev.
	AccountModeMavioDev AccountMode = "mavio.dev"
)

// Valid reports whether m is a known mode.
func (m AccountMode) Valid() bool {
	return m == AccountModeUnset || m == AccountModeStandalone || m == AccountModeMavioDev
}

// AccountSettings are how the server signs people in. They are kept apart
// from the ServerSettings, which administrators change while the server
// runs: the mode is chosen once, at setup.
type AccountSettings struct {
	Mode AccountMode
	// Administrator is the Mavio account that claimed a server in
	// mavio.dev mode; empty while it is unclaimed.
	Administrator string `json:",omitempty"`
	// Cursor is the position of the last change from mavio.dev the server
	// applied.
	Cursor int64 `json:",omitempty"`
}

// Claimed reports whether a server in mavio.dev mode has an
// administrator.
func (a *AccountSettings) Claimed() bool {
	return a.Mode == AccountModeMavioDev && a.Administrator != ""
}

// Validate checks the mode and that only a server in mavio.dev mode is
// claimed.
func (a *AccountSettings) Validate() error {
	switch {
	case !a.Mode.Valid():
		return fmt.Errorf("%w: unknown account mode %q", ErrInvalid, a.Mode)
	case a.Mode != AccountModeMavioDev && (a.Administrator != "" || a.Cursor != 0):
		return fmt.Errorf("%w: only a server in mavio.dev mode is claimed", ErrInvalid)
	case a.Cursor < 0:
		return fmt.Errorf("%w: negative change cursor", ErrInvalid)
	}
	return nil
}
