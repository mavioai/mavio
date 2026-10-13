// Package avistaz is the providers of the AvistaZ network's private
// trackers, AvistaZ and CinemaZ, ported from Bazarr's avistaz_network:
// the subtitles of the release a video was grabbed from, listed on the
// release's page, with the cookies of a browser logged in to the
// tracker.
package avistaz

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// Names of the providers.
const (
	Name        = "avistaz"
	CinemaZName = "cinemaz"
)

// InfoURLKey is the key of a video's ExternalIDs holding the URL of the
// tracker's page of the release the video was grabbed from, as Bazarr's
// video.info_url.
const InfoURLKey = "info_url"

// languageNames are the tracker's languages, as Bazarr's
// supported_languages_names.
var languageNames = []string{
	"Abkhazian", "Afar", "Afrikaans", "Akan", "Albanian", "Amharic", "Arabic", "Aragonese", "Armenian", "Assamese",
	"Avaric", "Avestan", "Aymara", "Azerbaijani", "Bambara", "Bashkir", "Basque", "Belarusian", "Bengali",
	"Bihari languages", "Bislama", "Bokmål, Norwegian", "Bosnian", "Brazilian Portuguese", "Breton", "Bulgarian",
	"Burmese", "Cantonese", "Catalan", "Central Khmer", "Chamorro", "Chechen", "Chichewa", "Chinese", "Church Slavic",
	"Chuvash", "Cornish", "Corsican", "Cree", "Croatian", "Czech", "Danish", "Dhivehi", "Dutch", "Dzongkha", "English",
	"Esperanto", "Estonian", "Ewe", "Faroese", "Fijian", "Filipino", "Finnish", "French", "Fulah", "Gaelic", "Galician",
	"Ganda", "Georgian", "German", "Greek", "Guarani", "Gujarati", "Haitian", "Hausa", "Hebrew", "Herero", "Hindi",
	"Hiri Motu", "Hungarian", "Icelandic", "Ido", "Igbo", "Indonesian", "Interlingua", "Interlingue", "Inuktitut",
	"Inupiaq", "Irish", "Italian", "Japanese", "Javanese", "Kalaallisut", "Kannada", "Kanuri", "Kashmiri", "Kazakh",
	"Kikuyu", "Kinyarwanda", "Kirghiz", "Komi", "Kongo", "Korean", "Kuanyama", "Kurdish", "Lao", "Latin", "Latvian",
	"Limburgan", "Lingala", "Lithuanian", "Luba-Katanga", "Luxembourgish", "Macedonian", "Malagasy", "Malay",
	"Malayalam", "Maltese", "Mandarin", "Manx", "Maori", "Marathi", "Marshallese", "Mongolian", "Moore", "Nauru",
	"Navajo", "Ndebele, North", "Ndebele, South", "Ndonga", "Nepali", "Northern Sami", "Norwegian", "Norwegian Nynorsk",
	"Occitan (post 1500)", "Ojibwa", "Oriya", "Oromo", "Ossetian", "Pali", "Panjabi", "Persian", "Polish", "Portuguese",
	"Pushto", "Quechua", "Romanian", "Romansh", "Rundi", "Russian", "Samoan", "Sango", "Sanskrit", "Sardinian",
	"Serbian", "Shona", "Sichuan Yi", "Sindhi", "Sinhala", "Slovak", "Slovenian", "Somali", "Sotho, Southern", "Spanish",
	"Sundanese", "Swahili", "Swati", "Swedish", "Tagalog", "Tahitian", "Tajik", "Tamil", "Tatar", "Telugu", "Thai",
	"Tibetan", "Tigrinya", "Tongan", "Tsonga", "Tswana", "Turkish", "Turkmen", "Twi", "Uighur", "Ukrainian", "Urdu",
	"Uzbek", "Venda", "Vietnamese", "Volapük", "Walloon", "Welsh", "Western Frisian", "Wolof", "Xhosa", "Yiddish",
	"Yoruba", "Zhuang", "Zulu",
}

func languages() []string {
	var out []string
	for _, n := range languageNames {
		if tag := sitekit.LanguageByName(n); tag != "" && !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}

// Definition returns AvistaZ's definition.
func Definition() provider.Definition {
	return definition(Name, "AvistaZ", New)
}

// CinemaZDefinition returns CinemaZ's definition.
func CinemaZDefinition() provider.Definition {
	return definition(CinemaZName, "CinemaZ", NewCinemaZ)
}

func definition(name, display string, newFunc func(provider.Env, json.RawMessage) (provider.Provider, error)) provider.Definition {
	return provider.Definition{
		Name: name, DisplayName: display,
		Settings: []provider.Setting{
			{Name: "cookies", Label: "Cookies, you must copy the whole value from your browser", Type: provider.Text},
			{Name: "user_agent", Label: "User-Agent, e.g., Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:95.0) Gecko/20100101 Firefox/95.0", Type: provider.Text},
		},
		Languages:      languages(),
		HashVerifiable: true, Movies: true, Episodes: true,
		New: newFunc,
	}
}

type settings struct {
	Cookies   string `json:"cookies"`
	UserAgent string `json:"user_agent"`
}

// New makes AvistaZ from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	return newTracker(env, raw, Name, "https://avistaz.to/")
}

// NewCinemaZ makes CinemaZ from its settings.
func NewCinemaZ(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	return newTracker(env, raw, CinemaZName, "https://cinemaz.to/")
}

func newTracker(env provider.Env, raw json.RawMessage, name, server string) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	u, _ := url.Parse(server)
	return &Provider{env: env, s: s, name: name, server: u, pause: 2 * time.Second, cache: map[string][]byte{}}, nil
}

// Provider is a tracker of the AvistaZ network.
type Provider struct {
	env    provider.Env
	s      settings
	name   string
	server *url.URL
	// pause is Bazarr's pause once the cookies are checked.
	pause time.Duration

	mu   sync.Mutex
	sess *site.Session
	// cache keeps the downloads of the session, as Bazarr's archive cache:
	// the episodes of a season pack are picked from one download.
	cache map[string][]byte
	order []string
}

// maxCached bounds the downloads kept.
const maxCached = 8

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.server = u.JoinPath("/") }

// session returns the session, checking the cookies first, as Bazarr's
// initialize.
func (p *Provider) session(ctx context.Context) (*site.Session, error) {
	if p.sess != nil {
		return p.sess, nil
	}
	s := site.New(p.env.HTTP)
	s.Header.Set("User-Agent", cmp.Or(p.s.UserAgent, site.UserAgent))
	if p.s.Cookies != "" {
		s.Jar().SetCookies(p.server, sitekit.ParseCookies(p.s.Cookies))
		rules := p.server.JoinPath("rules")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rules.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", p.server.String())
		r, err := s.Do(req)
		// Expired cookies redirect to the login, or are refused.
		if r != nil && (r.Status == http.StatusNotFound || r.Status == http.StatusForbidden || r.URL.Path != rules.Path) {
			return nil, provider.Errorf(provider.AuthenticationError, "cookies not valid anymore")
		}
		if err != nil {
			return nil, err
		}
		p.env.Log.DebugContext(ctx, p.name+": cookies valid")
		if err := site.Sleep(ctx, p.pause); err != nil {
			return nil, err
		}
	}
	p.sess = s
	return s, nil
}

type row struct {
	sub       provider.Subtitle
	archive   bool
	likes     int
	dislikes  int
	downloads int
}

// Search implements provider.Provider.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	info := v.ExternalIDs[InfoURLKey]
	if info == "" || !strings.HasPrefix(info, p.server.String()) {
		p.env.Log.DebugContext(ctx, p.name+": video not downloaded from the tracker, skipped")
		return nil, nil
	}
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.Get(ctx, info, nil)
	if r != nil && r.Status == http.StatusNotFound {
		p.env.Log.DebugContext(ctx, p.name+": release page not found, the release might have been removed")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "release: %w", err)
	}
	table := doc.First("#content-area > div.block > div.table-responsive > table > tbody")
	if !table.Valid() {
		return nil, provider.Errorf(provider.ProviderError, "unexpected HTML page layout: no release data table found")
	}
	release := map[string]site.Node{}
	for _, tr := range table.Children() {
		if tr.Data != "tr" {
			continue
		}
		if cells := tdChildren(tr); len(cells) > 0 {
			var value site.Node
			if len(cells) > 1 {
				value = cells[1]
			}
			release[cells[0].Text()] = value
		}
	}
	subsTable := release["Subtitles"].First("table")
	if !subsTable.Valid() {
		p.env.Log.DebugContext(ctx, p.name+": no subtitles found")
		return nil, nil
	}
	var columns []string
	for _, th := range subsTable.First("thead").All("th") {
		columns = append(columns, th.Text())
	}
	releaseName := strings.TrimSpace(release["Title"].Text())
	var rows []row
	for _, tr := range subsTable.First("tbody").Children() {
		if tr.Data != "tr" {
			continue
		}
		cols := map[string]site.Node{}
		var rating site.Node
		for i, td := range tdChildren(tr) {
			if i >= len(columns) {
				break
			}
			cols[columns[i]] = td
			if columns[i] == "" && !rating.Valid() && td.First(".subtitle-likes").Valid() {
				rating = td
			}
		}
		tag := sitekit.LanguageByName(strings.TrimSpace(cols["Language"].Text()))
		link := cols["Download"].First("a").Attr("href")
		if tag == "" || link == "" {
			continue
		}
		lang, ok := sitekit.Match(langs, tag)
		if !ok {
			continue
		}
		uploader := strings.TrimSpace(cols["Uploader"].Text())
		downloads, _ := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(cols["Downloads"].Text()), ",", ""))
		var likes, dislikes int
		if rating.Valid() {
			l, err1 := strconv.Atoi(strings.TrimSpace(rating.First(`a[data-like="like"] .count`).Text()))
			d, err2 := strconv.Atoi(strings.TrimSpace(rating.First(`a[data-like="dislike"] .count`).Text()))
			if err1 == nil && err2 == nil {
				likes, dislikes = l, d
			}
		}
		filename := path.Base(link)
		isArchive := strings.HasSuffix(filename, ".zip") || strings.HasSuffix(filename, ".rar")
		// A descriptive release tells the subtitles apart in manual searches.
		ext := "SRT"
		switch {
		case strings.HasSuffix(link, ".zip"):
			ext = "ZIP"
		case strings.HasSuffix(link, ".rar"):
			ext = "RAR"
		}
		ratingText := strconv.Itoa(downloads) + "↓"
		if likes != 0 || dislikes != 0 {
			ratingText = strconv.Itoa(likes) + "\U0001f44d " + strconv.Itoa(dislikes) + "\U0001f44e"
		}
		rows = append(rows, row{
			sub: provider.Subtitle{
				ID: link, Language: lang,
				ReleaseNames: []string{releaseName + " [" + cmp.Or(uploader, "unknown") + "] " + ext + " " + ratingText},
				// The subtitles are of the release the video was grabbed from.
				Matches:  []string{"hash"},
				PageLink: info, Uploader: uploader, Downloads: downloads,
			},
			archive: isArchive, likes: likes, dislikes: dislikes, downloads: downloads,
		})
	}
	// Season packs first, then by rating, then by downloads.
	slices.SortStableFunc(rows, func(a, b row) int {
		return cmp.Or(
			cmp.Compare(boolInt(!a.archive), boolInt(!b.archive)),
			cmp.Compare(b.likes-b.dislikes, a.likes-a.dislikes),
			cmp.Compare(b.downloads, a.downloads),
		)
	})
	out := make([]provider.Subtitle, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.sub)
	}
	return out, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// tdChildren returns a row's own cells.
func tdChildren(tr site.Node) []site.Node {
	var out []site.Node
	for _, c := range tr.Children() {
		if c.Data == "td" {
			out = append(out, c)
		}
	}
	return out
}

var (
	seasonEpisode = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,3})`)
	episodeOnly   = regexp.MustCompile(`\.E(\d{1,3})\.`)
	subExtensions = []string{".srt", ".ass", ".ssa", ".vtt"}
)

// Download implements provider.Provider: the episode's subtitle of a
// season pack is picked by its name, as Bazarr's _pick_from_cache.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	data, err := p.fetch(ctx, id)
	if err != nil {
		return provider.File{}, err
	}
	name := path.Base(id)
	if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".rar") {
		data = archive.FixLineEndings(data)
		return provider.File{Data: data, Format: cmp.Or(archive.Detect(data), strings.TrimPrefix(path.Ext(name), "."))}, nil
	}
	if v != nil && v.Episode {
		if f, ok := pick(data, v.Number); ok {
			return f, nil
		}
	}
	return archive.Extract(data, archive.WantFor(v, provider.Language{}), provider.Language{})
}

// pick returns the subtitle of an episode from an archive, by the names
// of its subtitle files in order.
func pick(data []byte, episode int) (provider.File, bool) {
	a, err := archive.Open(data)
	if err != nil {
		return provider.File{}, false
	}
	var names []string
	for _, n := range a.Names() {
		if slices.Contains(subExtensions, strings.ToLower(path.Ext(n))) {
			names = append(names, n)
		}
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(path.Base(a), path.Base(b)) })
	for _, re := range []*regexp.Regexp{seasonEpisode, episodeOnly} {
		for _, n := range names {
			m := re.FindStringSubmatch(path.Base(n))
			if m == nil {
				continue
			}
			if e, _ := strconv.Atoi(m[len(m)-1]); e != episode {
				continue
			}
			sub, err := a.Read(n)
			if err != nil {
				return provider.File{}, false
			}
			sub = archive.FixLineEndings(sub)
			return provider.File{Data: sub, Format: cmp.Or(archive.Detect(sub), strings.TrimPrefix(strings.ToLower(path.Ext(n)), "."))}, true
		}
	}
	return provider.File{}, false
}

// fetch downloads a subtitle or archive once per session.
func (p *Provider) fetch(ctx context.Context, link string) ([]byte, error) {
	if data, ok := p.cache[link]; ok {
		return data, nil
	}
	s, err := p.session(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.Get(ctx, link, nil)
	if err != nil {
		return nil, err
	}
	if len(p.order) >= maxCached {
		delete(p.cache, p.order[0])
		p.order = p.order[1:]
	}
	p.cache[link] = r.Body
	p.order = append(p.order, link)
	return r.Body, nil
}
