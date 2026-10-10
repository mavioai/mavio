package gazelle

import (
	"errors"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// FreeleechToken is when freeleech tokens are used for downloads.
type FreeleechToken int

// The uses of freeleech tokens.
const (
	// TokenNever does not use tokens.
	TokenNever FreeleechToken = iota
	// TokenPreferred uses a token when it can.
	TokenPreferred
	// TokenRequired only downloads releases a token can be used for.
	TokenRequired
)

// Settings are a Gazelle site's settings; each site takes those its
// definition's fields name.
type Settings struct {
	indexer.UserPassSettings
	// Cookie logs in instead of the user name and password, for accounts
	// with two-factor authentication.
	Cookie            string         `json:"cookie,omitempty"`
	UseFreeleechToken FreeleechToken `json:"use_freeleech_token,omitempty"`
	FreeleechOnly     bool           `json:"freeleech_only,omitempty"`
	ExcludeScene      bool           `json:"exclude_scene,omitempty"`
}

// validate checks the settings: the user name and password, or for sites
// taking one, the cookie instead.
func (s Settings) validate(cookie bool) error {
	if !cookie {
		return s.Validate()
	}
	if strings.TrimSpace(s.Cookie) != "" {
		return nil
	}
	if strings.TrimSpace(s.Username) == "" && strings.TrimSpace(s.Password) == "" {
		return errors.New("cookie: must not be empty without a username and password")
	}
	return s.Validate()
}

// The fields of Gazelle sites' settings.
var (
	cookieField = indexer.Field{
		Name: "cookie", Label: "Cookie", Type: indexer.FieldText, Secret: true,
		HelpText: "Use the Cookie field only if 2FA is enabled for your account, leave it empty otherwise.",
	}
	freeleechTokenField = indexer.Field{
		Name: "use_freeleech_token", Label: "Use Freeleech Tokens", Type: indexer.FieldSelect,
		HelpText: "When to use freeleech tokens", Default: int(TokenNever),
		Options: []indexer.Option{
			{Value: int(TokenNever), Label: "Never"},
			{Value: int(TokenPreferred), Label: "Preferred"},
			{Value: int(TokenRequired), Label: "Required"},
		},
	}
	excludeSceneField = indexer.Field{
		Name: "exclude_scene", Label: "Exclude SCENE", Type: indexer.FieldCheckbox,
		HelpText: "Exclude SCENE releases from results",
	}
)

// freeleechOnlyField returns the field searching only freeleech releases.
func freeleechOnlyField(help string) indexer.Field {
	return indexer.Field{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: help}
}

// fields returns the fields of a site's settings.
func fields(links []string, cookie bool, extra ...indexer.Field) []indexer.Field {
	f := []indexer.Field{indexer.BaseURLField(links), indexer.UsernameField, indexer.PasswordField}
	if cookie {
		f = append(f, cookieField)
	}
	f = append(f, freeleechTokenField)
	return append(f, extra...)
}
