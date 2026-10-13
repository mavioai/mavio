// Package sitekit is what the providers of sites with logins, cookies and
// captchas share, as Bazarr's subliminal utilities and pitchers: names
// sanitized for comparison, cookies pasted from a browser, logins kept
// across restarts in the provider's data folder, reCAPTCHA site keys found
// on pages, and the requested languages a site's language matches.
package sitekit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// DefaultCharacters are those Sanitize replaces with spaces, as
// subliminal_patch's sanitize.
const DefaultCharacters = "-:()."

var spaces = regexp.MustCompile(`\s+`)

// Sanitize returns a string for comparison, as subliminal_patch's
// sanitize: "-:()." replaced with spaces, quotes removed, spaces
// collapsed, trimmed and lower case.
func Sanitize(s string) string { return SanitizeChars(s, DefaultCharacters) }

// SanitizeChars is Sanitize replacing other characters with spaces.
func SanitizeChars(s, chars string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case strings.ContainsRune(chars, r):
			return ' '
		case strings.ContainsRune("'´`’", r):
			return -1
		}
		return r
	}, s)
	return strings.ToLower(strings.TrimSpace(spaces.ReplaceAllString(s, " ")))
}

var bracketed = regexp.MustCompile(`\[[\w|.]+\]`)

// SanitizeReleaseGroup returns a release group for comparison, as
// subliminal's sanitize_release_group: bracketed parts removed, trimmed
// and upper case.
func SanitizeReleaseGroup(s string) string {
	return strings.ToUpper(strings.TrimSpace(bracketed.ReplaceAllString(s, "")))
}

// EquivalentReleaseGroups returns the release groups that are the same
// as a sanitized one, as subliminal's get_equivalent_release_groups.
func EquivalentReleaseGroups(group string) []string {
	for _, g := range [][]string{{"FRAMESTOR", "W4NK3R", "BHDSTUDIO"}, {"LOL", "DIMENSION"}, {"ASAP", "IMMERSE", "FLEET"}, {"AVS", "SVA"}} {
		if slices.Contains(g, group) {
			return g
		}
	}
	return []string{group}
}

// FramerateEqual reports whether frame rates are the same, as Bazarr's
// framerate_equal: 23.976, 23.98 and 24 are.
func FramerateEqual(a, b float64) bool {
	if a == b {
		return true
	}
	film := func(f float64) bool { return f == 23.976 || f == 23.98 || f == 24 }
	return film(a) && film(b)
}

// ParseCookies parses cookies pasted from a browser, "a=1; b=2", as
// Python's SimpleCookie.
func ParseCookies(s string) []*http.Cookie {
	var out []*http.Cookie
	for part := range strings.SplitSeq(s, ";") {
		name, value, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		out = append(out, &http.Cookie{Name: name, Value: value, Path: "/"})
	}
	return out
}

// Load reads a provider's state of a name from its data folder into v;
// false when there is none or no folder.
func Load(dir, name string, v any) (bool, error) {
	if dir == "" {
		return false, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load %s: %w", name, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("load %s: %w", name, err)
	}
	return true, nil
}

// Save writes a provider's state of a name to its data folder; nothing
// without a folder.
func Save(dir, name string, v any) error {
	if dir == "" {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("save %s: %w", name, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save %s: %w", name, err)
	}
	f, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return fmt.Errorf("save %s: %w", name, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return fmt.Errorf("save %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("save %s: %w", name, err)
	}
	if err := os.Rename(f.Name(), filepath.Join(dir, name+".json")); err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("save %s: %w", name, err)
	}
	return nil
}

// Remove deletes a provider's state of a name.
func Remove(dir, name string) error {
	if dir == "" {
		return nil
	}
	if err := os.Remove(filepath.Join(dir, name+".json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", name, err)
	}
	return nil
}

// Login is a site's session kept across restarts, as Bazarr's
// store_verification: its cookies and the User-Agent they were given to.
type Login struct {
	Cookies   map[string]string `json:"cookies"`
	UserAgent string            `json:"user_agent,omitempty"`
	Saved     time.Time         `json:"saved"`
}

// SaveLogin keeps the cookies a jar has for a site, as Bazarr's
// store_verification.
func SaveLogin(dir, name string, jar http.CookieJar, u *url.URL, userAgent string, now time.Time) error {
	l := Login{Cookies: map[string]string{}, UserAgent: userAgent, Saved: now}
	for _, c := range jar.Cookies(u) {
		l.Cookies[c.Name] = c.Value
	}
	return Save(dir, name, l)
}

// LoadLogin puts a site's kept cookies younger than maxAge into a jar, as
// Bazarr's load_verification; false when there are none.
func LoadLogin(dir, name string, jar http.CookieJar, u *url.URL, maxAge time.Duration, now time.Time) (Login, bool, error) {
	var l Login
	ok, err := Load(dir, name, &l)
	if err != nil || !ok || len(l.Cookies) == 0 || now.Sub(l.Saved) > maxAge {
		return Login{}, false, err
	}
	var cookies []*http.Cookie
	for k, v := range l.Cookies {
		cookies = append(cookies, &http.Cookie{Name: k, Value: v, Path: "/"})
	}
	jar.SetCookies(u, cookies)
	return l, true, nil
}

var (
	recaptchaKey = regexp.MustCompile(`g-recaptcha.+?data-sitekey="(.+?)"`)
	executeKey   = regexp.MustCompile(`grecaptcha.execute\('(.+?)',`)
)

// HasRecaptcha reports whether a page asks for a reCAPTCHA.
func HasRecaptcha(page string) bool {
	return strings.Contains(page, "g-recaptcha") || strings.Contains(page, "grecaptcha")
}

// RecaptchaKey returns the reCAPTCHA site key of a page and the form
// field its answer is posted in, as Bazarr's providers find them; "" for
// none.
func RecaptchaKey(page string) (field, key string) {
	if m := recaptchaKey.FindStringSubmatch(page); m != nil {
		return "g-recaptcha-response", m[1]
	}
	if m := executeKey.FindStringSubmatch(page); m != nil {
		return "recaptcha_response", m[1]
	}
	return "", ""
}

// Match returns the requested language a site's language is, as
// babelfish's equality: of the same tag and not forced, since the sites
// do not tell forced subtitles.
func Match(langs []provider.Language, tag string) (provider.Language, bool) {
	for _, l := range langs {
		if !l.Forced && strings.EqualFold(l.Tag, tag) {
			return provider.Language{Tag: l.Tag}, true
		}
	}
	return provider.Language{}, false
}

// names are the English names of languages, as babelfish's and pycountry's,
// that provider.FromName does not know or gives deprecated codes of.
var names = map[string]string{
	"akan": "ak", "bengali": "bn", "bokmål, norwegian": "nb", "brazilian portuguese": "pt-BR", "cantonese": "yue",
	"central khmer": "km", "chichewa": "ny", "dhivehi": "dv", "filipino": "fil", "gaelic": "gd", "haitian": "ht",
	"hebrew": "he", "indonesian": "id", "javanese": "jv", "kirghiz": "ky", "limburgan": "li", "mandarin": "zh",
	"moore": "mos", "ndebele, north": "nd", "ndebele, south": "nr", "norwegian": "no", "occitan (post 1500)": "oc",
	"oriya": "or", "ossetian": "os", "panjabi": "pa", "pushto": "ps", "sotho, southern": "st", "tagalog": "tl",
	"twi": "tw", "uighur": "ug", "yiddish": "yi",
}

// LanguageByName returns the BCP 47 tag of a language's English name, as
// babelfish's and pycountry's names; "" when the name is unknown.
func LanguageByName(name string) string {
	if tag, ok := names[strings.ToLower(strings.TrimSpace(name))]; ok {
		return tag
	}
	return provider.FromName(name)
}

// Captcha returns the environment's captcha solver, or a configuration
// failure when none is configured, as Bazarr's pitchers without an
// anti-captcha provider.
func Captcha(env provider.Env, site string) (provider.CaptchaSolver, error) {
	if env.Captcha == nil {
		return nil, provider.Errorf(provider.ConfigurationError, "%s requires a captcha solving service", site)
	}
	return env.Captcha, nil
}
