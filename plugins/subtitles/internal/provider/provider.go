package provider

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Language is a language of subtitles with their kind.
type Language struct {
	// Tag is a BCP 47 tag, such as "en", "pt-BR" or "zh-Hant".
	Tag             string
	Forced          bool
	HearingImpaired bool
}

// Video describes the video subtitles are searched for, as the server
// gives it.
type Video struct {
	Episode           bool
	Title             string
	AlternativeTitles []string
	Year              int
	ExternalIDs       map[string]string
	Series            string
	AlternativeSeries []string
	SeriesExternalIDs map[string]string
	Season, Number    int
	AbsoluteNumber    int
	OriginalSeries    bool
	Special           bool
	ReleaseName       string
	Source            string
	ReleaseGroup      string
	Resolution        string
	VideoCodec        string
	AudioCodec        string
	Edition           string
	StreamingService  string
	FrameRate         float64
	FileName          string
	FileSize          int64
	FileHash          string
	AudioLanguages    []string
	OriginalLanguage  string
	// AudioURL serves the video's audio through the host API (see
	// Env.Host); "" when the server does not read it, as on cloud drives.
	AudioURL string
	// AudioStreams are the video's audio streams.
	AudioStreams []AudioStream
}

// AudioStream is an audio stream of a video.
type AudioStream struct {
	// Index is the stream's among the audio streams, from 0, which the
	// audio URL's "stream" parameter takes.
	Index int
	// Language is the stream's tag as the file holds it, such as "eng",
	// "ger" or "und"; "" when it has none.
	Language string
}

// IMDbID returns the IMDb ID of the movie, or of the series of an episode.
func (v *Video) IMDbID() string {
	if v.Episode {
		return v.SeriesExternalIDs["imdb"]
	}
	return v.ExternalIDs["imdb"]
}

// Subtitle is a subtitle a provider found.
type Subtitle struct {
	// ID is the provider's, passed to Download.
	ID       string
	Language Language
	// ReleaseNames are the releases the subtitle was made for.
	ReleaseNames []string
	// Matches are the properties the provider knows the subtitle matches,
	// as subliminal names them.
	Matches []string
	// HashVerifiable and HearingImpairedVerifiable override the
	// provider's, when set.
	HashVerifiable, HearingImpairedVerifiable *bool
	Format                                    string
	PageLink, Uploader, Comment               string
	Downloads                                 int
}

// File is a downloaded subtitle.
type File struct {
	Data []byte
	// Format is "srt", "ass", "ssa", "vtt" or "sub"; "" when unknown.
	Format   string
	Language Language
}

// Provider searches a subtitle site.
type Provider interface {
	// Search lists the subtitles of a video in the languages.
	Search(ctx context.Context, v *Video, langs []Language) ([]Subtitle, error)
	// Download returns a subtitle Search found; v is the video searched
	// for, which providers of packs pick the episode's subtitle with.
	Download(ctx context.Context, v *Video, id string) (File, error)
}

// Env is what providers are made with.
type Env struct {
	HTTP *http.Client
	Now  func() time.Time
	Log  *slog.Logger
	// DataDir is the provider's folder for sessions and caches, "" for
	// none.
	DataDir string
	// Captcha solves captchas with the configured service; nil when none
	// is configured.
	Captcha CaptchaSolver
	// RetryWait is how long providers wait before retrying a request.
	RetryWait time.Duration
	// Host reaches the server's host API, such as videos' audio URLs.
	Host *http.Client
}

// CaptchaSolver solves the captchas of sites with a captcha solving
// service, as Bazarr's pitchers.
type CaptchaSolver interface {
	// ReCaptchaV2 returns the response token of a reCAPTCHA v2 on a page,
	// invisible or not.
	ReCaptchaV2(ctx context.Context, pageURL, siteKey string, invisible bool) (string, error)
	// Image returns the text of a captcha image.
	Image(ctx context.Context, image []byte) (string, error)
}

// Definition is a provider the plugin implements.
type Definition struct {
	// Name is stable, as Bazarr names the provider: "opensubtitlescom".
	Name        string
	DisplayName string
	Settings    []Setting
	// Languages are the BCP 47 tags of the provider's languages; empty for
	// any.
	Languages []string
	// HashVerifiable and HearingImpairedVerifiable are as subliminal's
	// subtitles' flags.
	HashVerifiable, HearingImpairedVerifiable bool
	Movies, Episodes                          bool
	// Throttle overrides the default throttling durations by failure.
	Throttle map[Kind]func(now time.Time) time.Duration
	// New makes the provider from its settings, a JSON object.
	New func(env Env, settings json.RawMessage) (Provider, error)
}

// SettingType is the type of a setting's value.
type SettingType int

// Setting types.
const (
	Text SettingType = iota
	Password
	Number
	Checkbox
	Select
)

// Setting is a setting a provider takes.
type Setting struct {
	Name, Label, Help string
	Type              SettingType
	// Default is the value when unset; nil for none.
	Default any
	// Options are the choices of a select.
	Options  []string
	Advanced bool
}
