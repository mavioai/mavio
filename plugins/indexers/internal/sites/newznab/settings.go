package newznab

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Settings are a Newznab or Torznab indexer's settings.
type Settings struct {
	BaseURL string `json:"base_url"`
	// APIPath is the API's path under the base URL; "/api" when "".
	APIPath string `json:"api_path,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
	// AdditionalParameters are appended to searches' URLs, such as
	// "&attrs=poster".
	AdditionalParameters string `json:"additional_parameters,omitempty"`
	// VIPExpiration is when the account's VIP status expires, as
	// "2006-01-02".
	VIPExpiration string `json:"vip_expiration,omitempty"`
}

// apiURL returns the URL of the API, without parameters.
func (s Settings) apiURL() string {
	path := s.APIPath
	if path == "" {
		path = "/api"
	}
	return strings.TrimRight(s.BaseURL, "/") + strings.TrimRight(path, "/")
}

// APIKeySites are the Newznab indexers that cannot be used without an
// API key, by part of their URL.
var APIKeySites = []string{
	"nzbs.org",
	"nzb.life",
	"dognzb.cr",
	"nzbplanet.net",
	"nzbid.org",
	"nzbndx.com",
	"nzbindex.in",
}

// FieldError is a setting that is not valid.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

var (
	additionalParameters = regexp.MustCompile(`(&.+?=.+?)+`)
	urlBase              = regexp.MustCompile(`(?i)^/?https?://[-_a-z0-9.]+`)
)

// Validate checks a Newznab indexer's settings.
func (s Settings) Validate() error {
	return ValidateSettings(s, APIKeySites)
}

// ValidateSettings checks settings, requiring an API key for the sites
// given by part of their URL, as Prowlarr's NewznabSettingsValidator does.
func ValidateSettings(s Settings, apiKeySites []string) error {
	var errs []error
	fail := func(field, format string, args ...any) {
		errs = append(errs, &FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	if !validRootURL(s.BaseURL) {
		fail("base_url", "must be valid URL that starts with http(s)://")
	}
	if urlBase.MatchString(s.APIPath) {
		fail("api_path", "must be a valid URL path (ie: '/api')")
	}
	lower := strings.ToLower(s.BaseURL)
	for _, site := range apiKeySites {
		if s.BaseURL != "" && strings.Contains(lower, site) && strings.TrimSpace(s.APIKey) == "" {
			fail("api_key", "must not be empty")
			break
		}
	}
	if strings.TrimSpace(s.AdditionalParameters) != "" && !additionalParameters.MatchString(s.AdditionalParameters) {
		fail("additional_parameters", "must be parameters such as &attrs=poster")
	}
	if strings.TrimSpace(s.VIPExpiration) != "" {
		if _, err := time.Parse(time.DateOnly, strings.TrimSpace(s.VIPExpiration)); err != nil {
			fail("vip_expiration", "correctly formatted date is required")
		}
	}
	return errors.Join(errs...)
}

// validRootURL reports whether s is an absolute HTTP URL, as Prowlarr's
// ValidRootUrl checks.
func validRootURL(s string) bool {
	if strings.TrimSpace(s) == "" || strings.TrimSpace(s) != s {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
