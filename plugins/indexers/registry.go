package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/avistaz"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/beyondhd"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/bithdtv"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/broadcasthenet"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/cardigann"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/filelist"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/gazelle"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/gazellegames"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/hdbits"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/hdtorrents"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/headphones"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/immortalseed"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/iptorrents"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/knaben"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/libble"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/mteamtp"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/myanonamouse"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/nebulance"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/newznab"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/norbits"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/nzbindex"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/passthepopcorn"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/pixelhd"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/pretome"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/revolutiontt"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/rutracker"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/scenehd"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/shazbat"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/speedapp"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/speedcd"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentbytes"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentday"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentpotato"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentrss"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentscsv"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torrentsyndikat"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/torznab"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/uniotaku"
	"github.com/mavioai/mavio/plugins/indexers/internal/sites/xspeeds"
)

// codeDefinitions are the definitions implemented in code: the generic
// protocols with Prowlarr's presets of them, and the sites Prowlarr
// implements in code.
func codeDefinitions() []indexer.Definition {
	defs := protocolDefinitions()
	defs = append(defs, avistaz.Definitions...)
	defs = append(defs, gazelle.Definitions...)
	defs = append(defs, speedapp.Definitions...)
	defs = append(defs,
		beyondhd.Definition,
		bithdtv.Definition,
		broadcasthenet.Definition,
		filelist.Definition,
		gazellegames.Definition,
		hdbits.Definition,
		hdtorrents.Definition,
		headphones.Definition,
		immortalseed.Definition,
		iptorrents.Definition,
		knaben.Definition,
		libble.Definition,
		mteamtp.Definition,
		myanonamouse.Definition,
		nebulance.Definition,
		norbits.Definition,
		nzbindex.Definition,
		passthepopcorn.Definition,
		pixelhd.Definition,
		pretome.Definition,
		revolutiontt.Definition,
		rutracker.Definition,
		scenehd.Definition,
		shazbat.Definition,
		speedcd.Definition,
		torrentbytes.Definition,
		torrentday.Definition,
		torrentpotato.Definition,
		torrentscsv.Definition,
		torrentsyndikat.Definition,
		uniotaku.Definition,
		xspeeds.Definition,
	)
	return defs
}

// preset is an indexer speaking a generic protocol, which Prowlarr lists
// as a definition of its own.
type preset struct {
	name, description, language, url, apiPath string
}

// newznabPresets are Prowlarr's Newznab indexers, the last generic.
var newznabPresets = []preset{
	{name: "abNZB", url: "https://abnzb.com"},
	{name: "altHUB", url: "https://api.althub.co.za"},
	{name: "AnimeTosho (Usenet)", url: "https://feed.animetosho.org"},
	{name: "DOGnzb", url: "https://api.dognzb.cr"},
	{name: "DrunkenSlug", url: "https://drunkenslug.com"},
	{name: "GingaDADDY", url: "https://www.gingadaddy.com"},
	{name: "Miatrix", url: "https://www.miatrix.com"},
	{name: "Newz69", url: "https://newz69.keagaming.com"},
	{name: "NinjaCentral", url: "https://ninjacentral.co.za"},
	{name: "Nzb.life", url: "https://api.nzb.life"},
	{name: "NZBCat", url: "https://nzb.cat"},
	{name: "NZBFinder", url: "https://nzbfinder.ws"},
	{name: "NZBgeek", url: "https://api.nzbgeek.info"},
	{name: "NzbNoob", url: "https://www.nzbnoob.com"},
	{name: "NZBNDX", url: "https://www.nzbndx.com"},
	{name: "NzbPlanet", url: "https://api.nzbplanet.net"},
	{name: "NZBStars", url: "https://nzbstars.com"},
	{name: "Tabula Rasa", url: "https://www.tabula-rasa.pw", apiPath: "/api/v1/api"},
	{name: "Generic Newznab"},
}

// torznabPresets are Prowlarr's Torznab indexers, the last generic.
var torznabPresets = []preset{
	{name: "AnimeTosho", description: "Anime NZB/DDL mirror", url: "https://feed.animetosho.org"},
	{name: "Torrent Network", description: "Torrent Network (TN) is a GERMAN Private site for TV / MOVIES / GENERAL", language: "de-DE", url: "https://tntracker.org", apiPath: "/api/torznab/api"},
	{name: "Generic Torznab", description: "A Newznab-like api for torrents."},
}

// protocolDefinitions returns the definitions of Newznab, Torznab and
// Torrent RSS feeds. The generic Newznab and Torznab definitions have the
// IDs "newznab" and "torznab"; presets are named by a slug of their names.
func protocolDefinitions() []indexer.Definition {
	var defs []indexer.Definition
	for _, p := range newznabPresets {
		d := protocolDefinition(p, "newznab", "Newznab is an API search specification for Usenet", indexer.ProtocolUsenet)
		d.New = func(data []byte) (indexer.Site, error) {
			s, err := newznabSettings(data, p)
			if err != nil {
				return nil, err
			}
			return newznab.New(s)
		}
		defs = append(defs, d)
	}
	for _, p := range torznabPresets {
		d := protocolDefinition(p, "torznab", "A Newznab-like api for torrents.", indexer.ProtocolTorrent)
		d.New = func(data []byte) (indexer.Site, error) {
			s, err := newznabSettings(data, p)
			if err != nil {
				return nil, err
			}
			return torznab.New(s)
		}
		defs = append(defs, d)
	}
	defs = append(defs, indexer.Definition{
		ID:          "torrentrss",
		Name:        "Torrent RSS Feed",
		Description: "Generic RSS Feed containing torrents",
		Language:    "en-US",
		Protocol:    indexer.ProtocolTorrent,
		Privacy:     indexer.Public,
		Fields: []indexer.Field{
			{Name: "base_url", Label: "Full RSS Feed URL", Type: indexer.FieldText, HelpText: "To sync to your apps you will need to include the 8000(Other) in the Sync Categories"},
			{Name: "cookie", Label: "Cookie", Type: indexer.FieldText, Secret: true, HelpText: "If your site requires a login cookie to access the RSS, you'll have to retrieve it via a browser."},
			{Name: "allow_zero_size", Label: "Allow Zero Size", Type: indexer.FieldCheckbox, HelpText: "Enabling this will allow you to use feeds that don't specify release size, but be careful, size related checks will not be performed."},
			{Name: "default_release_size", Label: "Default Release Size", Type: indexer.FieldNumber, HelpText: "Add a default size for feeds with missing sizes, in MB.", Advanced: true},
		},
		New: func(data []byte) (indexer.Site, error) {
			var s torrentrss.Settings
			if err := indexer.DecodeSettings(data, &s); err != nil {
				return nil, err
			}
			return torrentrss.New(s)
		},
	})
	return defs
}

func protocolDefinition(p preset, generic, description string, protocol indexer.Protocol) indexer.Definition {
	id := slug(p.name)
	if p.url == "" {
		id = generic
	}
	if p.description != "" {
		description = p.description
	}
	language := p.language
	if language == "" {
		language = "en-US"
	}
	apiPath := p.apiPath
	if apiPath == "" {
		apiPath = "/api"
	}
	d := indexer.Definition{
		ID: id, Name: p.name, Description: description, Language: language,
		Protocol: protocol, Privacy: indexer.Private,
		Fields: []indexer.Field{
			{Name: "base_url", Label: "URL", Type: indexer.FieldText},
			{Name: "api_path", Label: "API Path", Type: indexer.FieldText, Default: apiPath, Advanced: true, HelpText: "Path to the api, usually /api"},
			{Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true, HelpText: "Site API Key"},
			{Name: "additional_parameters", Label: "Additional Parameters", Type: indexer.FieldText, Advanced: true, HelpText: "Additional Newznab parameters"},
			{Name: "vip_expiration", Label: "VIP Expiration", Type: indexer.FieldText, HelpText: "Enter date (yyyy-mm-dd) for VIP Expiration or blank, Prowlarr will notify 1 week from expiration of VIP"},
		},
	}
	if p.url != "" {
		d.Links = []string{p.url}
		d.Fields[0].Default = p.url
	}
	return d
}

// newznabSettings reads the settings of a Newznab or Torznab indexer,
// those the preset gives filling those left out.
func newznabSettings(data []byte, p preset) (newznab.Settings, error) {
	s := newznab.Settings{BaseURL: p.url, APIPath: p.apiPath}
	if err := indexer.DecodeSettings(data, &s); err != nil {
		return s, err
	}
	if s.BaseURL == "" {
		s.BaseURL = p.url
	}
	return s, nil
}

// slug returns the ID of a preset's name: lower case, letters and digits
// kept, runs of others made a dash.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	return b.String()
}

// cardigannDefinition returns the indexer definition of a Cardigann
// definition, as Prowlarr's Cardigann indexer lists its definitions:
// named by its file, with its settings as fields.
func cardigannDefinition(def *cardigann.Definition, log *slog.Logger) indexer.Definition {
	privacy := indexer.SemiPrivate
	switch def.Type {
	case "private":
		privacy = indexer.Private
	case "public":
		privacy = indexer.Public
	}
	fields := []indexer.Field{indexer.BaseURLField(def.Links)}
	for _, s := range def.Settings {
		fields = append(fields, cardigannField(def, s))
	}
	if def.Login != nil && def.Login.Captcha != nil {
		fields = append(fields, indexer.Field{Name: "CAPTCHA", Label: "CAPTCHA", Type: indexer.FieldText, HelpText: "The text of the login's captcha"})
	}
	return indexer.Definition{
		ID: def.File, Name: def.Name, Description: def.Description, Language: def.Language,
		Protocol: indexer.ProtocolTorrent, Privacy: privacy,
		Links: def.Links, LegacyLinks: def.LegacyLinks,
		Fields: fields,
		New: func(data []byte) (indexer.Site, error) {
			s, err := cardigannSettings(def, data)
			if err != nil {
				return nil, err
			}
			return cardigann.New(def, s, log)
		},
	}
}

// The help shown for Cardigann's informational settings, as Prowlarr's
// IndexerResourceMapper gives it.
const (
	cookieHelp = "Log in to the tracker with your browser, keeping the session (\"Remember me\") and not tying it to your IP address; " +
		"open the site's search page, open the browser's developer tools on the Network tab, refresh the page, " +
		"select the first request and copy the whole value of its 'cookie:' request header."
	userAgentHelp   = "From the same request you copied the cookie from, copy the whole value of its 'user-agent:' request header."
	flareSolverrFmt = "This site may use Cloudflare DDoS Protection, therefore it requires a FlareSolverr proxy to access it."
	category8000Fmt = "%s does not return categories in its search results. To sync to your apps, include 8000(Other) in your Apps' Sync Categories."
)

func cardigannField(def *cardigann.Definition, s cardigann.SettingsField) indexer.Field {
	f := indexer.Field{Name: s.Name, Label: s.Label, Type: indexer.FieldText}
	switch s.Type {
	case "password":
		f.Secret = true
	case "checkbox":
		f.Type = indexer.FieldCheckbox
		f.Default = strings.EqualFold(s.Default, "true")
		return f
	case "select":
		f.Type = indexer.FieldSelect
		keys := make([]string, 0, len(s.Options))
		labels := map[string]string{}
		for _, o := range s.Options {
			keys = append(keys, o.Key)
			labels[o.Key] = o.Value
		}
		slices.Sort(keys)
		for _, k := range keys {
			f.Options = append(f.Options, indexer.Option{Value: k, Label: labels[k]})
		}
		if slices.Contains(keys, s.Default) {
			f.Default = s.Default
		}
		return f
	case "multi-select":
		f.Type = indexer.FieldMultiSelect
		for _, o := range s.Options {
			f.Options = append(f.Options, indexer.Option{Value: o.Key, Label: o.Value})
		}
		if len(s.Defaults) > 0 {
			f.Default = s.Defaults
		}
		return f
	case "info":
		f.Type = indexer.FieldInfo
		f.HelpText = s.Default
		return f
	case "info_cookie":
		f.Type, f.Label, f.HelpText = indexer.FieldInfo, "How to get the Cookie", cookieHelp
		return f
	case "info_useragent":
		f.Type, f.Label, f.HelpText = indexer.FieldInfo, "How to get the User-Agent", userAgentHelp
		return f
	case "info_flaresolverr":
		f.Type, f.Label, f.HelpText = indexer.FieldInfo, "FlareSolverr Info", flareSolverrFmt
		return f
	case "info_category_8000":
		f.Type, f.Label, f.HelpText = indexer.FieldInfo, "About "+def.Name+" Categories", fmt.Sprintf(category8000Fmt, def.Name)
		return f
	}
	if s.Name == "cookie" {
		f.Secret = true
	}
	if s.Default != "" {
		f.Default = s.Default
	}
	return f
}

// cardigannSettings reads the settings of an indexer of a Cardigann
// definition: base_url, and the definition's settings by name.
func cardigannSettings(def *cardigann.Definition, data []byte) (cardigann.Settings, error) {
	var m map[string]any
	if err := indexer.DecodeSettings(data, &m); err != nil {
		return cardigann.Settings{}, err
	}
	s := cardigann.Settings{Definition: def.File, Fields: map[string]any{}}
	if u, ok := m["base_url"].(string); ok {
		s.BaseURL = u
	}
	known := map[string]bool{"CAPTCHA": true}
	for _, f := range def.Settings {
		known[f.Name] = true
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if k == "base_url" {
			continue
		}
		if !known[k] {
			return s, fmt.Errorf("unknown setting %q of definition %s", k, def.File)
		}
		s.Fields[k] = m[k]
	}
	return s, nil
}

// definitionsByID indexes definitions by ID, those implemented in code
// first: a Cardigann definition with the ID of one of them is left out.
func definitionsByID(code []indexer.Definition, defs []*cardigann.Definition, log *slog.Logger) ([]indexer.Definition, map[string]indexer.Definition) {
	all := slices.Clone(code)
	byID := map[string]indexer.Definition{}
	for _, d := range code {
		byID[d.ID] = d
	}
	for _, def := range defs {
		if _, ok := byID[def.File]; ok {
			log.Warn("cardigann definition shadowed by a site implemented in code", "definition", def.File)
			continue
		}
		d := cardigannDefinition(def, log)
		byID[d.ID] = d
		all = append(all, d)
	}
	return all, byID
}

// jsonValue returns v as JSON, "" for nil.
func jsonValue(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
