package indexer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FieldType is how a setting is entered.
type FieldType int

// The field types.
const (
	FieldText FieldType = iota + 1
	FieldNumber
	FieldCheckbox
	FieldSelect
	FieldMultiSelect
	FieldInfo
)

// Field is a setting of an indexer.
type Field struct {
	// Name is the setting's key in the settings' JSON.
	Name     string
	Label    string
	HelpText string
	Type     FieldType
	// Default is the value when unset; nil for none.
	Default any
	// Options are the choices of selects.
	Options  []Option
	Advanced bool
	// Secret settings, such as passwords, are never shown back.
	Secret bool
}

// Option is a choice of a select.
type Option struct {
	Value any
	Label string
}

// Definition is a kind of indexer: a site, or a protocol many sites
// speak.
type Definition struct {
	// ID is stable, such as "newznab" or "redacted".
	ID          string
	Name        string
	Description string
	Language    string
	Protocol    Protocol
	Privacy     Privacy
	// Links are the site's addresses, the first by default.
	Links []string
	// LegacyLinks are former addresses, replaced by the first link.
	LegacyLinks []string
	Fields      []Field
	// Limits are the indexer's limits unless its settings give others.
	Limits Limits
	// New returns an indexer site of the definition from its settings'
	// JSON.
	New func(settings []byte) (Site, error)
}

// The fields most sites take.
var (
	UsernameField = Field{Name: "username", Label: "Username", Type: FieldText}
	PasswordField = Field{Name: "password", Label: "Password", Type: FieldText, Secret: true}
	APIKeyField   = Field{Name: "api_key", Label: "API Key", Type: FieldText, Secret: true}
	CookieField   = Field{
		Name: "cookie", Label: "Cookie", Type: FieldText, Secret: true,
		HelpText: "Login cookie from the website",
	}
)

// BaseURLField returns the field of a site's address, one of its links.
func BaseURLField(links []string) Field {
	f := Field{Name: "base_url", Label: "Base URL", Type: FieldSelect, HelpText: "Select which base URL to use for requests to the site"}
	for _, l := range links {
		f.Options = append(f.Options, Option{Value: l, Label: l})
	}
	if len(links) > 0 {
		f.Default = links[0]
	}
	return f
}

// BaseSettings are the settings every site takes.
type BaseSettings struct {
	// BaseURL is the site's address; its first link when "".
	BaseURL string `json:"base_url,omitempty"`
}

// URL returns the site's address ending with a slash: the settings' unless
// it is a legacy link, else the first link.
func (s BaseSettings) URL(links, legacy []string) string {
	u := s.BaseURL
	if u == "" || containsFold(legacy, u) {
		if len(links) == 0 {
			return ""
		}
		u = links[0]
	}
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

// UserPassSettings are the settings of sites logging in with a user name
// and password.
type UserPassSettings struct {
	BaseSettings
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Validate checks that the user name and password are set.
func (s UserPassSettings) Validate() error {
	if strings.TrimSpace(s.Username) == "" {
		return fmt.Errorf("username: must not be empty")
	}
	if strings.TrimSpace(s.Password) == "" {
		return fmt.Errorf("password: must not be empty")
	}
	return nil
}

// DecodeSettings reads settings' JSON into v; empty JSON leaves v as is.
func DecodeSettings(data []byte, v any) error {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}
	return nil
}
