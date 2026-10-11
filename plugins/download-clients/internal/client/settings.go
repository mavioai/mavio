package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// Server is where a client listens, as the host, port, SSL and URL base
// settings of Radarr's clients give it.
type Server struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	UseSSL  bool   `json:"use_ssl"`
	URLBase string `json:"url_base"`
}

// BaseURL returns the client's address as Radarr's
// HttpRequestBuilder.BuildBaseUrl builds it, without a trailing slash.
func (s Server) BaseURL() string {
	scheme := "http"
	if s.UseSSL {
		scheme = "https"
	}
	base := strings.TrimSpace(s.URLBase)
	if base != "" && !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return fmt.Sprintf("%s://%s%s", scheme, net.JoinHostPort(s.Host, strconv.Itoa(s.Port)), strings.TrimRight(base, "/"))
}

// IsLocalhost reports whether the host is this machine.
func (s Server) IsLocalhost() bool { return IsLocalhost(s.Host) }

// IsLocalhost reports whether a host is this machine, as Radarr's clients
// tell: 127.0.0.1, ::1 or localhost.
func IsLocalhost(host string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]")) {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

// urlBasePattern matches URL bases that are whole URLs, which Radarr's
// ValidUrlBase refuses.
var urlBasePattern = regexp.MustCompile(`(?i)^/?https?://[-_a-z0-9.]+`)

// ValidateURLBase checks that the URL base of a setting, named field, is a
// path rather than a URL, as Radarr's ValidUrlBase; example is a valid
// one, such as "/sabnzbd". A blank URL base is valid.
func ValidateURLBase(field, urlBase, example string) error {
	if urlBasePattern.MatchString(strings.TrimSpace(urlBase)) {
		return fmt.Errorf("%s: must be a valid URL path (ie: '%s')", field, example)
	}
	return nil
}

// Validate checks the host and port.
func (s Server) Validate() error {
	switch {
	case strings.TrimSpace(s.Host) == "":
		return fmt.Errorf("host is required")
	case s.Port <= 0 || s.Port > 65535:
		return fmt.Errorf("port %d is out of range", s.Port)
	}
	return nil
}

// ServerFields are the fields of Server, with a client's default port and
// URL base.
func ServerFields(port int, urlBase string) []Field {
	fields := []Field{
		{Name: "host", Label: "Host", Type: FieldText, Default: "localhost"},
		{Name: "port", Label: "Port", Type: FieldNumber, Default: port},
		{Name: "use_ssl", Label: "Use SSL", Type: FieldCheckbox, Default: false, HelpText: "Use a secure connection"},
	}
	if urlBase != "-" {
		fields = append(fields, Field{
			Name: "url_base", Label: "URL Base", Type: FieldText, Default: urlBase, Advanced: true,
			HelpText: "Adds a prefix to the client's URL, such as http://[host]:[port]/[urlBase]",
		})
	}
	return fields
}

// Categories are the categories, labels or folders a client puts the
// downloads of each kind in, and those it moves them to once imported.
type Categories struct {
	MovieCategory         string `json:"movie_category"`
	MovieImportedCategory string `json:"movie_imported_category"`
	TVCategory            string `json:"tv_category"`
	TVImportedCategory    string `json:"tv_imported_category"`
}

// Category returns the category of a kind; that of movies for zero.
func (c Categories) Category(k Kind) string {
	if k == Series {
		return c.TVCategory
	}
	return c.MovieCategory
}

// ImportedCategory returns the category imported downloads of a kind are
// moved to; empty for none.
func (c Categories) ImportedCategory(k Kind) string {
	if k == Series {
		return c.TVImportedCategory
	}
	return c.MovieImportedCategory
}

// HasPostImportCategory reports whether imported downloads of a kind move
// to a category of their own.
func (c Categories) HasPostImportCategory(k Kind) bool {
	imported := c.ImportedCategory(k)
	return imported != "" && imported != c.Category(k)
}

// Match returns the kind whose category matches, by a test of each
// category; zero when both match, and false when neither does.
func (c Categories) Match(in func(category string) bool) (Kind, bool) {
	movie, series := in(c.MovieCategory), in(c.TVCategory)
	switch {
	case movie && series:
		return 0, true
	case movie:
		return Movie, true
	case series:
		return Series, true
	}
	return 0, false
}

// CategoryFields are the fields of Categories with Radarr's and Sonarr's
// defaults; label names what a category is in the client, such as
// "Category" or "Label".
func CategoryFields(label, movieDefault, tvDefault string, imported bool) []Field {
	fields := []Field{
		{
			Name: "movie_category", Label: "Movie " + label, Type: FieldText, Default: movieDefault,
			HelpText: fmt.Sprintf("Adding a %s for movies avoids conflicts with unrelated downloads", strings.ToLower(label)),
		},
		{
			Name: "tv_category", Label: "Series " + label, Type: FieldText, Default: tvDefault,
			HelpText: fmt.Sprintf("Adding a %s for series avoids conflicts with unrelated downloads", strings.ToLower(label)),
		},
	}
	if imported {
		fields = append(fields,
			Field{
				Name: "movie_imported_category", Label: "Post-Import Movie " + label, Type: FieldText, Advanced: true,
				HelpText: fmt.Sprintf("%s for movies to set after importing; leave blank to keep the same one", label),
			},
			Field{
				Name: "tv_imported_category", Label: "Post-Import Series " + label, Type: FieldText, Advanced: true,
				HelpText: fmt.Sprintf("%s for series to set after importing; leave blank to keep the same one", label),
			},
		)
	}
	return fields
}

// Priorities are the priorities a client gives downloads, by kind and by
// whether the movie was released, or the episode aired, lately.
type Priorities struct {
	RecentMoviePriority int `json:"recent_movie_priority"`
	OlderMoviePriority  int `json:"older_movie_priority"`
	RecentTVPriority    int `json:"recent_tv_priority"`
	OlderTVPriority     int `json:"older_tv_priority"`
}

// Priority returns the priority of a release.
func (p Priorities) Priority(k Kind, recent bool) int {
	switch {
	case k == Series && recent:
		return p.RecentTVPriority
	case k == Series:
		return p.OlderTVPriority
	case recent:
		return p.RecentMoviePriority
	}
	return p.OlderMoviePriority
}

// PriorityFields are the fields of Priorities, choices of options with a
// default.
func PriorityFields(options []Option, def any) []Field {
	field := func(name, label, help string) Field {
		return Field{Name: name, Label: label, Type: FieldSelect, Options: options, Default: def, HelpText: help}
	}
	return []Field{
		field("recent_movie_priority", "Recent Movie Priority", "Priority to use when grabbing movies released within the last 21 days"),
		field("older_movie_priority", "Older Movie Priority", "Priority to use when grabbing movies released over 21 days ago"),
		field("recent_tv_priority", "Recent Priority", "Priority to use when grabbing episodes that aired within the last 14 days"),
		field("older_tv_priority", "Older Priority", "Priority to use when grabbing episodes that aired over 14 days ago"),
	}
}

// Directories are the folders a client puts downloads of each kind in,
// instead of its default; empty for the default.
type Directories struct {
	MovieDirectory string `json:"movie_directory"`
	TVDirectory    string `json:"tv_directory"`
}

// Directory returns the folder of a kind.
func (d Directories) Directory(k Kind) string {
	if k == Series {
		return d.TVDirectory
	}
	return d.MovieDirectory
}

// DirectoryFields are the fields of Directories.
func DirectoryFields(client string) []Field {
	help := fmt.Sprintf("Optional location to put downloads in, leave blank to use the default %s location", client)
	return []Field{
		{Name: "movie_directory", Label: "Movie Directory", Type: FieldText, Advanced: true, HelpText: help},
		{Name: "tv_directory", Label: "Series Directory", Type: FieldText, Advanced: true, HelpText: help},
	}
}

// The default categories of movies and series, as Radarr's "radarr" and
// Sonarr's "tv-sonarr".
const (
	DefaultMovieCategory = "movies-mavio"
	DefaultTVCategory    = "tv-mavio"
)

// Decode reads a client's settings' JSON into s, which holds the
// defaults; unknown settings are rejected.
func Decode(settings []byte, s any) error {
	dec := json.NewDecoder(bytes.NewReader(settings))
	dec.DisallowUnknownFields()
	if err := dec.Decode(s); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}
	return nil
}
