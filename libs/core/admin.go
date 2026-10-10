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
	// PluginCatalogs are the URLs of the plugin catalogs plugins are
	// installed from.
	PluginCatalogs []string
	// PasswordResetPlugin is the ID of the plugin that delivers the PINs
	// resetting forgotten passwords; empty when users cannot reset them.
	PasswordResetPlugin string
	UpdatedAt           time.Time
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
	// and key in CertificatePath and KeyPath (PEM files).
	HTTPSPort                int
	CertificatePath, KeyPath string
	// LocalDiscovery answers discovery requests from clients on the local
	// network.
	LocalDiscovery bool
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
		Network: NetworkSettings{LocalDiscovery: true},
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
	case n.HTTPSPort > 0 && (n.CertificatePath == "" || n.KeyPath == ""):
		return fmt.Errorf("%w: HTTPS needs a certificate and a key", ErrInvalid)
	}
	for _, c := range s.PluginCatalogs {
		if u, err := url.Parse(c); err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
			return fmt.Errorf("%w: plugin catalog %q is not an http or https URL", ErrInvalid, c)
		}
	}
	return nil
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
	Limit       int // zero means MaxPageSize
	Offset      int
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
