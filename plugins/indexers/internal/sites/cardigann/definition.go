package cardigann

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Definition is a Cardigann definition: a YAML description of an
// indexer site, as Prowlarr's CardigannDefinition is.
type Definition struct {
	// File is the name of the definition's file without extension, by
	// which indexers name it.
	File            string            `yaml:"-"`
	ID              string            `yaml:"id"`
	Settings        []SettingsField   `yaml:"settings"`
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Type            string            `yaml:"type"`
	Language        string            `yaml:"language"`
	Encoding        string            `yaml:"encoding"`
	RequestDelay    *float64          `yaml:"requestDelay"`
	Links           []string          `yaml:"links"`
	LegacyLinks     []string          `yaml:"legacylinks"`
	FollowRedirect  bool              `yaml:"followredirect"`
	TestLinkTorrent *bool             `yaml:"testLinkTorrent"`
	Certificates    []string          `yaml:"certificates"`
	Caps            CapabilitiesBlock `yaml:"caps"`
	Login           *LoginBlock       `yaml:"login"`
	Ratio           *RatioBlock       `yaml:"ratio"`
	Search          *SearchBlock      `yaml:"search"`
	Download        *DownloadBlock    `yaml:"download"`
}

// SettingsField is a setting a definition asks for.
type SettingsField struct {
	Name     string          `yaml:"name"`
	Type     string          `yaml:"type"`
	Label    string          `yaml:"label"`
	Default  string          `yaml:"default"`
	Defaults []string        `yaml:"defaults"`
	Options  Ordered[string] `yaml:"options"`
}

// CategoryMapping maps a site's category to a Newznab category.
type CategoryMapping struct {
	ID      string `yaml:"id"`
	Cat     string `yaml:"cat"`
	Desc    string `yaml:"desc"`
	Default bool   `yaml:"default"`
}

// CapabilitiesBlock is a definition's categories and search modes.
type CapabilitiesBlock struct {
	Categories       Ordered[string]   `yaml:"categories"`
	CategoryMappings []CategoryMapping `yaml:"categorymappings"`
	Modes            Ordered[[]string] `yaml:"modes"`
	AllowRawSearch   bool              `yaml:"allowrawsearch"`
}

// CaptchaBlock is the captcha of a login form.
type CaptchaBlock struct {
	Type     string  `yaml:"type"`
	Selector string  `yaml:"selector"`
	Image    *string `yaml:"image"`
	Input    string  `yaml:"input"`
}

// LoginBlock is how a site is logged in to.
type LoginBlock struct {
	Path              string                 `yaml:"path"`
	SubmitPath        string                 `yaml:"submitpath"`
	Cookies           []string               `yaml:"cookies"`
	Method            string                 `yaml:"method"`
	Form              string                 `yaml:"form"`
	Selectors         bool                   `yaml:"selectors"`
	Inputs            Ordered[string]        `yaml:"inputs"`
	SelectorInputs    Ordered[SelectorBlock] `yaml:"selectorinputs"`
	GetSelectorInputs Ordered[SelectorBlock] `yaml:"getselectorinputs"`
	Error             []ErrorBlock           `yaml:"error"`
	Test              *PageTestBlock         `yaml:"test"`
	Captcha           *CaptchaBlock          `yaml:"captcha"`
	Headers           Ordered[[]string]      `yaml:"headers"`
}

// ErrorBlock is an element telling a request failed.
type ErrorBlock struct {
	Path     string         `yaml:"path"`
	Selector string         `yaml:"selector"`
	Message  *SelectorBlock `yaml:"message"`
}

// SelectorBlock selects a value from an element or a JSON object.
type SelectorBlock struct {
	Selector  *string         `yaml:"selector"`
	Optional  bool            `yaml:"optional"`
	Default   string          `yaml:"default"`
	Text      *string         `yaml:"text"`
	Attribute *string         `yaml:"attribute"`
	Remove    *string         `yaml:"remove"`
	Filters   []Filter        `yaml:"filters"`
	Case      Ordered[string] `yaml:"case"`
}

// Filter is a filter of selected values.
type Filter struct {
	Name string
	// Args are the filter's arguments: one for a scalar, none for null.
	Args []string
	// List is whether the arguments are a list.
	List bool
}

// UnmarshalYAML implements yaml.Unmarshaler, taking scalar arguments and
// lists of them.
func (f *Filter) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		Name string    `yaml:"name"`
		Args yaml.Node `yaml:"args"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	f.Name = raw.Name
	switch raw.Args.Kind {
	case 0:
	case yaml.ScalarNode:
		if raw.Args.Tag != "!!null" {
			f.Args = []string{raw.Args.Value}
		}
	case yaml.SequenceNode:
		f.List = true
		for _, a := range raw.Args.Content {
			if a.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: filter %s takes scalar arguments", a.Line, f.Name)
			}
			f.Args = append(f.Args, a.Value)
		}
	default:
		return fmt.Errorf("line %d: filter %s takes scalar arguments", raw.Args.Line, f.Name)
	}
	return nil
}

// arg returns the filter's single argument; ok is false for none.
func (f *Filter) arg() (string, bool) {
	if f.List || len(f.Args) == 0 {
		return "", false
	}
	return f.Args[0], true
}

// listArg returns the filter's i-th argument of a list.
func (f *Filter) listArg(i int) (string, error) {
	if !f.List || i >= len(f.Args) {
		return "", fmt.Errorf("filter %s needs %d arguments", f.Name, i+1)
	}
	return f.Args[i], nil
}

// PageTestBlock tells whether a page is that of a logged in user.
type PageTestBlock struct {
	Path     string `yaml:"path"`
	Selector string `yaml:"selector"`
}

// RatioBlock selects the user's ratio.
type RatioBlock struct {
	SelectorBlock `yaml:",inline"`
	Path          string `yaml:"path"`
}

// SearchBlock is how a site is searched and its results read.
type SearchBlock struct {
	Path                 string                 `yaml:"path"`
	Paths                []SearchPathBlock      `yaml:"paths"`
	Headers              Ordered[[]string]      `yaml:"headers"`
	KeywordsFilters      []Filter               `yaml:"keywordsfilters"`
	AllowEmptyInputs     bool                   `yaml:"allowEmptyInputs"`
	Inputs               Ordered[string]        `yaml:"inputs"`
	Error                []ErrorBlock           `yaml:"error"`
	PreprocessingFilters []Filter               `yaml:"preprocessingfilters"`
	Rows                 RowsBlock              `yaml:"rows"`
	Fields               Ordered[SelectorBlock] `yaml:"fields"`
}

// RowsBlock selects the rows of results.
type RowsBlock struct {
	SelectorBlock                   `yaml:",inline"`
	After                           int            `yaml:"after"`
	DateHeaders                     *SelectorBlock `yaml:"dateheaders"`
	Count                           *SelectorBlock `yaml:"count"`
	Multiple                        bool           `yaml:"multiple"`
	MissingAttributeEqualsNoResults bool           `yaml:"missingAttributeEqualsNoResults"`
}

// RequestBlock is a request.
type RequestBlock struct {
	Path           string          `yaml:"path"`
	Method         string          `yaml:"method"`
	Inputs         Ordered[string] `yaml:"inputs"`
	QuerySeparator *string         `yaml:"queryseparator"`
}

// separator returns the separator of the request's query arguments.
func (r *RequestBlock) separator() string {
	if r.QuerySeparator == nil {
		return "&"
	}
	return *r.QuerySeparator
}

// SearchPathBlock is a request of a search.
type SearchPathBlock struct {
	RequestBlock   `yaml:",inline"`
	Categories     []string       `yaml:"categories"`
	InheritInputs  *bool          `yaml:"inheritinputs"`
	FollowRedirect bool           `yaml:"followredirect"`
	Response       *ResponseBlock `yaml:"response"`
}

// inheritsInputs reports whether the search's inputs are added to the
// path's.
func (p *SearchPathBlock) inheritsInputs() bool {
	return p.InheritInputs == nil || *p.InheritInputs
}

// DownloadBlock is how a release's download is found.
type DownloadBlock struct {
	Selectors []SelectorField   `yaml:"selectors"`
	Method    string            `yaml:"method"`
	Before    *BeforeBlock      `yaml:"before"`
	InfoHash  *InfoHashBlock    `yaml:"infohash"`
	Headers   Ordered[[]string] `yaml:"headers"`
}

// InfoHashBlock selects a release's info hash and title, for a magnet
// link.
type InfoHashBlock struct {
	Hash              *SelectorField `yaml:"hash"`
	Title             *SelectorField `yaml:"title"`
	UseBeforeResponse bool           `yaml:"usebeforeresponse"`
}

// SelectorField selects a value from a page.
type SelectorField struct {
	Selector          string   `yaml:"selector"`
	Attribute         *string  `yaml:"attribute"`
	UseBeforeResponse bool     `yaml:"usebeforeresponse"`
	Filters           []Filter `yaml:"filters"`
}

// BeforeBlock is a request made before a download.
type BeforeBlock struct {
	RequestBlock `yaml:",inline"`
	PathSelector *SelectorField `yaml:"pathselector"`
}

// ResponseBlock is the kind of a search's response.
type ResponseBlock struct {
	Type             string  `yaml:"type"`
	NoResultsMessage *string `yaml:"noResultsMessage"`
}

// Entry is an entry of an ordered mapping.
type Entry[V any] struct {
	Key   string
	Value V
}

// Ordered is a YAML mapping in the order the definition gives it,
// repeated keys included.
type Ordered[V any] []Entry[V]

// UnmarshalYAML implements yaml.Unmarshaler.
func (o *Ordered[V]) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		if n.Tag == "!!null" {
			return nil
		}
		return fmt.Errorf("line %d: expected a mapping", n.Line)
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		var v V
		if err := n.Content[i+1].Decode(&v); err != nil {
			return err
		}
		*o = append(*o, Entry[V]{Key: n.Content[i].Value, Value: v})
	}
	return nil
}

// Get returns the value of the first entry with the key.
func (o Ordered[V]) Get(key string) (V, bool) {
	for _, e := range o {
		if e.Key == key {
			return e.Value, true
		}
	}
	var zero V
	return zero, false
}

// Parse reads a definition, completing it as Prowlarr does: with a
// username and password when it asks for no settings, UTF-8, logging in
// by form, and its single search path as a list of paths.
func Parse(data []byte) (*Definition, error) {
	var d Definition
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse definition: %w", err)
	}
	if d.Settings == nil {
		d.Settings = []SettingsField{
			{Name: "username", Label: "Username", Type: "text"},
			{Name: "password", Label: "Password", Type: "password"},
		}
	}
	if d.Encoding == "" {
		d.Encoding = "UTF-8"
	}
	if d.Login != nil {
		if d.Login.Method == "" {
			d.Login.Method = "form"
		}
		if d.Login.Captcha != nil && d.Login.Captcha.Image != nil {
			return nil, errors.New("parse definition: deprecated, please use login/captcha/selector instead of image")
		}
	}
	if d.Search == nil {
		return nil, fmt.Errorf("parse definition %s: no search", d.ID)
	}
	if d.Search.Path != "" {
		d.Search.Paths = append(d.Search.Paths, SearchPathBlock{RequestBlock: RequestBlock{Path: d.Search.Path}})
	}
	if len(d.Links) == 0 {
		return nil, fmt.Errorf("parse definition %s: no links", d.ID)
	}
	return &d, nil
}

// testsLinks reports whether downloads selected from pages are checked to
// be torrents.
func (d *Definition) testsLinks() bool {
	return d.TestLinkTorrent == nil || *d.TestLinkTorrent
}
