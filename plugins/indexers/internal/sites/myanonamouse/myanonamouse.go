package myanonamouse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://www.myanonamouse.net/"}

// searchTypes are the site's search types, by the index the settings
// keep.
var searchTypes = []struct{ value, label, hint string }{
	{"all", "All torrents", "Search everything"},
	{"active", "Only active", "Last update had 1+ seeders"},
	{"fl", "Freeleech", "Freeleech torrents"},
	{"fl-VIP", "Freeleech or VIP", "Freeleech or VIP torrents"},
	{"VIP", "VIP", "VIP torrents"},
	{"nVIP", "Not VIP", "Torrents not VIP"},
}

// languages are the site's languages, by their IDs.
var languages = []struct {
	id    int
	label string
}{
	{1, "English"},
	{17, "Afrikaans"},
	{32, "Arabic"},
	{35, "Bengali"},
	{51, "Bosnian"},
	{18, "Bulgarian"},
	{6, "Burmese"},
	{44, "Cantonese"},
	{19, "Catalan"},
	{2, "Chinese"},
	{49, "Croatian"},
	{20, "Czech"},
	{21, "Danish"},
	{22, "Dutch"},
	{61, "Estonian"},
	{39, "Farsi"},
	{23, "Finnish"},
	{36, "French"},
	{37, "German"},
	{26, "Greek"},
	{59, "Greek, Ancient"},
	{3, "Gujarati"},
	{27, "Hebrew"},
	{8, "Hindi"},
	{28, "Hungarian"},
	{63, "Icelandic"},
	{53, "Indonesian"},
	{56, "Irish"},
	{43, "Italian"},
	{38, "Japanese"},
	{12, "Javanese"},
	{5, "Kannada"},
	{41, "Korean"},
	{50, "Lithuanian"},
	{46, "Latin"},
	{62, "Latvian"},
	{33, "Malay"},
	{58, "Malayalam"},
	{57, "Manx"},
	{9, "Marathi"},
	{48, "Norwegian"},
	{45, "Polish"},
	{34, "Portuguese"},
	{52, "Brazilian Portuguese (BP)"},
	{14, "Punjabi"},
	{30, "Romanian"},
	{16, "Russian"},
	{24, "Scottish Gaelic"},
	{60, "Sanskrit"},
	{31, "Serbian"},
	{54, "Slovenian"},
	{4, "Spanish"},
	{55, "Castilian Spanish"},
	{40, "Swedish"},
	{29, "Tagalog"},
	{11, "Tamil"},
	{10, "Telugu"},
	{7, "Thai"},
	{42, "Turkish"},
	{25, "Ukrainian"},
	{15, "Urdu"},
	{13, "Vietnamese"},
	{47, "Other"},
}

func searchTypeOptions() []indexer.Option {
	opts := make([]indexer.Option, len(searchTypes))
	for i, t := range searchTypes {
		opts[i] = indexer.Option{Value: i, Label: t.label}
	}
	return opts
}

func languageOptions() []indexer.Option {
	opts := make([]indexer.Option, len(languages))
	for i, l := range languages {
		opts[i] = indexer.Option{Value: l.id, Label: l.label}
	}
	return opts
}

// Definition is MyAnonamouse.
var Definition = indexer.Definition{
	ID:          "myanonamouse",
	Name:        "MyAnonamouse",
	Description: "MyAnonaMouse (MAM) is a large ebook and audiobook tracker.",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "mam_id", Label: "Mam Id", Type: indexer.FieldText, Secret: true, HelpText: "Mam Session Id (Created Under Preferences -> Security)"},
		{Name: "search_type", Label: "Search Type", Type: indexer.FieldSelect, Default: 0, Options: searchTypeOptions(), HelpText: "Specify the desired search type"},
		{Name: "search_in_description", Label: "Search in description", Type: indexer.FieldCheckbox, HelpText: "Search text in the description"},
		{Name: "search_in_series", Label: "Search in series", Type: indexer.FieldCheckbox, HelpText: "Search text in the series"},
		{Name: "search_in_filenames", Label: "Search in filenames", Type: indexer.FieldCheckbox, HelpText: "Search text in the filenames"},
		{Name: "search_languages", Label: "Search Languages", Type: indexer.FieldMultiSelect, Options: languageOptions(), HelpText: "Specify the desired languages. If unspecified, all options are used."},
		{Name: "use_freeleech_wedge", Label: "Use Freeleech Wedges", Type: indexer.FieldCheckbox, HelpText: "Use freeleech wedges to make grabbed torrents personal freeleech"},
	},
	New: newSite,
}

// Settings are MyAnonamouse's settings.
type Settings struct {
	indexer.BaseSettings
	MamID               string `json:"mam_id,omitempty"`
	SearchType          int    `json:"search_type,omitempty"`
	SearchInDescription bool   `json:"search_in_description,omitempty"`
	SearchInSeries      bool   `json:"search_in_series,omitempty"`
	SearchInFilenames   bool   `json:"search_in_filenames,omitempty"`
	// SearchLanguages are IDs of languages.
	SearchLanguages   []int `json:"search_languages,omitempty"`
	UseFreeleechWedge bool  `json:"use_freeleech_wedge,omitempty"`
}

// Validate checks that the mam_id is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.MamID) == "" {
		return errors.New("mam_id: must not be empty")
	}
	return nil
}

// userClassLife is how long the user's class is kept.
const userClassLife = time.Hour

// Site is MyAnonamouse.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities

	mu        sync.Mutex
	userClass string
	classTill time.Time
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	for _, m := range []struct {
		id   string
		cat  *indexer.Category
		desc string
	}{
		{"13", indexer.AudioAudiobook, "AudioBooks"},
		{"14", indexer.BooksEBook, "E-Books"},
		{"15", indexer.AudioAudiobook, "Musicology"},
		{"16", indexer.AudioAudiobook, "Radio"},
		{"39", indexer.AudioAudiobook, "Audiobooks - Action/Adventure"},
		{"49", indexer.AudioAudiobook, "Audiobooks - Art"},
		{"50", indexer.AudioAudiobook, "Audiobooks - Biographical"},
		{"83", indexer.AudioAudiobook, "Audiobooks - Business"},
		{"51", indexer.AudioAudiobook, "Audiobooks - Computer/Internet"},
		{"97", indexer.AudioAudiobook, "Audiobooks - Crafts"},
		{"40", indexer.AudioAudiobook, "Audiobooks - Crime/Thriller"},
		{"41", indexer.AudioAudiobook, "Audiobooks - Fantasy"},
		{"106", indexer.AudioAudiobook, "Audiobooks - Food"},
		{"42", indexer.AudioAudiobook, "Audiobooks - General Fiction"},
		{"52", indexer.AudioAudiobook, "Audiobooks - General Non-Fic"},
		{"98", indexer.AudioAudiobook, "Audiobooks - Historical Fiction"},
		{"54", indexer.AudioAudiobook, "Audiobooks - History"},
		{"55", indexer.AudioAudiobook, "Audiobooks - Home/Garden"},
		{"43", indexer.AudioAudiobook, "Audiobooks - Horror"},
		{"99", indexer.AudioAudiobook, "Audiobooks - Humor"},
		{"84", indexer.AudioAudiobook, "Audiobooks - Instructional"},
		{"44", indexer.AudioAudiobook, "Audiobooks - Juvenile"},
		{"56", indexer.AudioAudiobook, "Audiobooks - Language"},
		{"45", indexer.AudioAudiobook, "Audiobooks - Literary Classics"},
		{"57", indexer.AudioAudiobook, "Audiobooks - Math/Science/Tech"},
		{"85", indexer.AudioAudiobook, "Audiobooks - Medical"},
		{"87", indexer.AudioAudiobook, "Audiobooks - Mystery"},
		{"119", indexer.AudioAudiobook, "Audiobooks - Nature"},
		{"88", indexer.AudioAudiobook, "Audiobooks - Philosophy"},
		{"58", indexer.AudioAudiobook, "Audiobooks - Pol/Soc/Relig"},
		{"59", indexer.AudioAudiobook, "Audiobooks - Recreation"},
		{"46", indexer.AudioAudiobook, "Audiobooks - Romance"},
		{"47", indexer.AudioAudiobook, "Audiobooks - Science Fiction"},
		{"53", indexer.AudioAudiobook, "Audiobooks - Self-Help"},
		{"89", indexer.AudioAudiobook, "Audiobooks - Travel/Adventure"},
		{"100", indexer.AudioAudiobook, "Audiobooks - True Crime"},
		{"108", indexer.AudioAudiobook, "Audiobooks - Urban Fantasy"},
		{"48", indexer.AudioAudiobook, "Audiobooks - Western"},
		{"111", indexer.AudioAudiobook, "Audiobooks - Young Adult"},
		{"60", indexer.BooksEBook, "Ebooks - Action/Adventure"},
		{"71", indexer.BooksEBook, "Ebooks - Art"},
		{"72", indexer.BooksEBook, "Ebooks - Biographical"},
		{"90", indexer.BooksEBook, "Ebooks - Business"},
		{"61", indexer.BooksComics, "Ebooks - Comics/Graphic novels"},
		{"73", indexer.BooksEBook, "Ebooks - Computer/Internet"},
		{"101", indexer.BooksEBook, "Ebooks - Crafts"},
		{"62", indexer.BooksEBook, "Ebooks - Crime/Thriller"},
		{"63", indexer.BooksEBook, "Ebooks - Fantasy"},
		{"107", indexer.BooksEBook, "Ebooks - Food"},
		{"64", indexer.BooksEBook, "Ebooks - General Fiction"},
		{"74", indexer.BooksEBook, "Ebooks - General Non-Fiction"},
		{"102", indexer.BooksEBook, "Ebooks - Historical Fiction"},
		{"76", indexer.BooksEBook, "Ebooks - History"},
		{"77", indexer.BooksEBook, "Ebooks - Home/Garden"},
		{"65", indexer.BooksEBook, "Ebooks - Horror"},
		{"103", indexer.BooksEBook, "Ebooks - Humor"},
		{"115", indexer.BooksEBook, "Ebooks - Illusion/Magic"},
		{"91", indexer.BooksEBook, "Ebooks - Instructional"},
		{"66", indexer.BooksEBook, "Ebooks - Juvenile"},
		{"78", indexer.BooksEBook, "Ebooks - Language"},
		{"67", indexer.BooksEBook, "Ebooks - Literary Classics"},
		{"79", indexer.BooksMags, "Ebooks - Magazines/Newspapers"},
		{"80", indexer.BooksTechnical, "Ebooks - Math/Science/Tech"},
		{"92", indexer.BooksEBook, "Ebooks - Medical"},
		{"118", indexer.BooksEBook, "Ebooks - Mixed Collections"},
		{"94", indexer.BooksEBook, "Ebooks - Mystery"},
		{"120", indexer.BooksEBook, "Ebooks - Nature"},
		{"95", indexer.BooksEBook, "Ebooks - Philosophy"},
		{"81", indexer.BooksEBook, "Ebooks - Pol/Soc/Relig"},
		{"82", indexer.BooksEBook, "Ebooks - Recreation"},
		{"68", indexer.BooksEBook, "Ebooks - Romance"},
		{"69", indexer.BooksEBook, "Ebooks - Science Fiction"},
		{"75", indexer.BooksEBook, "Ebooks - Self-Help"},
		{"96", indexer.BooksEBook, "Ebooks - Travel/Adventure"},
		{"104", indexer.BooksEBook, "Ebooks - True Crime"},
		{"109", indexer.BooksEBook, "Ebooks - Urban Fantasy"},
		{"70", indexer.BooksEBook, "Ebooks - Western"},
		{"112", indexer.BooksEBook, "Ebooks - Young Adult"},
		{"19", indexer.AudioAudiobook, "Guitar/Bass Tabs"},
		{"20", indexer.AudioAudiobook, "Individual Sheet"},
		{"24", indexer.AudioAudiobook, "Individual Sheet MP3"},
		{"126", indexer.AudioAudiobook, "Instructional Book with Video"},
		{"22", indexer.AudioAudiobook, "Instructional Media - Music"},
		{"113", indexer.AudioAudiobook, "Lick Library - LTP/Jam With"},
		{"114", indexer.AudioAudiobook, "Lick Library - Techniques/QL"},
		{"17", indexer.AudioAudiobook, "Music - Complete Editions"},
		{"26", indexer.AudioAudiobook, "Music Book"},
		{"27", indexer.AudioAudiobook, "Music Book MP3"},
		{"30", indexer.AudioAudiobook, "Sheet Collection"},
		{"31", indexer.AudioAudiobook, "Sheet Collection MP3"},
		{"127", indexer.AudioAudiobook, "Radio -  Comedy"},
		{"130", indexer.AudioAudiobook, "Radio - Drama"},
		{"128", indexer.AudioAudiobook, "Radio - Factual/Documentary"},
		{"132", indexer.AudioAudiobook, "Radio - Reading"},
	} {
		c.Categories.Add(m.id, m.cat, m.desc)
	}
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolTorrent,
		Privacy:            indexer.Private,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
		PageSize:           100,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// cookies returns the cookies requests carry, as Prowlarr's GetCookies
// does: the indexer's when they have a mam_id, else the settings' mam_id.
func (s *Site) cookies(ix *indexer.Indexer) map[string]string {
	if c := ix.Cookies(); strings.TrimSpace(c["mam_id"]) != "" {
		return c
	}
	return indexer.ParseCookieHeader("mam_id=" + s.settings.MamID)
}

// sanitizeRe matches what .NET's [^\w]+ does.
var sanitizeRe = regexp.MustCompile(`[^\p{L}\p{Mn}\p{Nd}\p{Pc}]+`)

// SearchRequests implements indexer.Site, as Prowlarr's
// MyAnonamouseRequestGenerator does: only book and basic searches.
func (s *Site) SearchRequests(ctx context.Context, ix *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	if q.Type != indexer.SearchBook && q.Type != indexer.SearchBasic {
		return &c, nil
	}
	term := strings.TrimSpace(sanitizeRe.ReplaceAllString(q.SanitizedTerm(), " "))
	if strings.TrimSpace(q.Term) != "" && term == "" {
		slog.DebugContext(ctx, "search term is empty after being sanitized, stopping search", "term", q.Term)
		return &c, nil
	}
	searchType := "all"
	if t := s.settings.SearchType; t > 0 && t < len(searchTypes) {
		searchType = searchTypes[t].value
	}
	var p indexer.Params
	p.Set("tor[text]", term)
	p.Set("tor[searchType]", searchType)
	p.Set("tor[srchIn][title]", "true")
	p.Set("tor[srchIn][author]", "true")
	p.Set("tor[srchIn][narrator]", "true")
	p.Set("tor[searchIn]", "torrents")
	p.Set("tor[sortType]", "default")
	p.Set("tor[startNumber]", strconv.Itoa(q.Offset))
	perPage := "100"
	if q.Limit > 0 {
		perPage = strconv.Itoa(q.Limit)
	}
	p.Set("perpage", perPage)
	p.Set("thumbnails", "1")
	p.Set("description", "1")
	if s.settings.SearchInDescription {
		p.Set("tor[srchIn][description]", "true")
	}
	if s.settings.SearchInSeries {
		p.Set("tor[srchIn][series]", "true")
	}
	if s.settings.SearchInFilenames {
		p.Set("tor[srchIn][filenames]", "true")
	}
	for i, l := range s.settings.SearchLanguages {
		p.Set(fmt.Sprintf("tor[browse_lang][%d]", i), strconv.Itoa(l))
	}
	if cats := s.caps.Categories.MapToTracker(q.Categories, false); len(cats) > 0 {
		for i, cat := range cats {
			p.Set(fmt.Sprintf("tor[cat][%d]", i), cat)
		}
	} else {
		p.Set("tor[cat][]", "0")
	}
	r := indexer.NewRequest(s.baseURL+"tor/js/loadSearchJSONbasic.php?"+p.Encode(), indexer.AcceptJSON)
	r.Cookies = s.cookies(ix)
	c.AddRequests(r)
	return &c, nil
}

// DownloadRequest implements indexer.DownloadSite: downloads carry the
// cookies searches do.
func (s *Site) DownloadRequest(_ context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	r.Cookies = s.cookies(ix)
	return r, nil
}

// torrent is a release of a search.
type torrent struct {
	ID                jsonx.Int64  `json:"id"`
	Title             jsonx.String `json:"title"`
	AuthorInfo        *string      `json:"author_info"`
	Description       jsonx.String `json:"description"`
	LanguageCode      jsonx.String `json:"lang_code"`
	Filetype          jsonx.String `json:"filetype"`
	VIP               jsonx.Bool   `json:"vip"`
	Free              jsonx.Bool   `json:"free"`
	PersonalFreeleech jsonx.Bool   `json:"personal_freeleech"`
	FreeVIP           jsonx.Bool   `json:"fl_vip"`
	Category          jsonx.String `json:"category"`
	Added             jsonx.String `json:"added"`
	Grabs             jsonx.Int    `json:"times_completed"`
	Seeders           jsonx.Int    `json:"seeders"`
	Leechers          jsonx.Int    `json:"leechers"`
	NumFiles          jsonx.Int    `json:"numfiles"`
	Size              jsonx.String `json:"size"`
}

// ParseResponse implements indexer.Site, as Prowlarr's MyAnonamouseParser
// does. Failures clear the cookies, which the settings' mam_id replaces.
func (s *Site) ParseResponse(ctx context.Context, ix *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("[403 Forbidden] - mam_id expired or invalid: %w", indexer.ErrAuth)
	}
	if r.StatusCode != http.StatusOK {
		ix.SetCookies(nil, time.Time{})
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !strings.Contains(r.Header.Get("Content-Type"), indexer.AcceptJSON) {
		ix.SetCookies(nil, time.Time{})
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp struct {
		Error   jsonx.String `json:"error"`
		Data    []torrent    `json:"data"`
		Message *string      `json:"message"`
	}
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("myanonamouse: %w", err)
	}
	if strings.HasPrefix(strings.ToLower(string(resp.Error)), "nothing returned, out of") {
		return nil, nil
	}
	if resp.Data == nil {
		msg := "Check the logs for more information."
		if resp.Message != nil {
			msg = *resp.Message
		}
		return nil, indexer.Errorf("Unexpected response content from indexer request: %s", msg)
	}
	// The cookies are those of the response, as Prowlarr's GetCookies
	// returns: the request's, updated by the response's.
	cookies := map[string]string{}
	for k, v := range r.Request.Cookies {
		cookies[k] = v
	}
	for k, v := range r.Cookies {
		cookies[k] = v
	}
	vip, err := s.hasUserVIP(ctx, ix, cookies)
	if err != nil {
		return nil, err
	}
	releases := make([]*indexer.Release, 0, len(resp.Data))
	for i := range resp.Data {
		rel, err := s.release(&resp.Data[i], vip)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	// The response renews the mam_id.
	ix.SetCookies(cookies, time.Time{})
	return releases, nil
}

func (s *Site) release(item *torrent, vip bool) (*indexer.Release, error) {
	id := strconv.FormatInt(int64(item.ID), 10)
	rel := &indexer.Release{
		Title:       string(item.Title),
		Description: string(item.Description),
		BookTitle:   string(item.Title),
	}
	if item.AuthorInfo != nil {
		authors, err := firstValues(*item.AuthorInfo, 5)
		if err != nil {
			return nil, fmt.Errorf("myanonamouse: author_info of %s: %w", id, err)
		}
		if author := strings.Join(authors, ", "); strings.TrimSpace(author) != "" {
			rel.Title += " by " + author
			rel.Author = author
		}
	}
	var flags []string
	if item.LanguageCode != "" {
		flags = append(flags, string(item.LanguageCode))
	}
	if item.Filetype != "" {
		flags = append(flags, strings.ToUpper(string(item.Filetype)))
	}
	if len(flags) > 0 {
		rel.Title += " [" + strings.Join(flags, " / ") + "]"
	}
	if item.VIP {
		rel.Title += " [VIP]"
	}
	free := bool(item.Free) || bool(item.PersonalFreeleech) || (vip && bool(item.FreeVIP))
	rel.DownloadURL = s.baseURL + "tor/download.php?tid=" + id
	if s.settings.UseFreeleechWedge && !free {
		rel.DownloadURL += "&fl=1"
	}
	rel.InfoURL = s.baseURL + "t/" + id
	rel.GUID = rel.InfoURL
	rel.Categories = indexer.IDs(s.caps.Categories.MapTracker(string(item.Category)))
	date, err := time.Parse(time.DateTime, string(item.Added))
	if err != nil {
		return nil, fmt.Errorf("myanonamouse: %w", err)
	}
	rel.PublishDate = date
	rel.Grabs = indexer.Ptr(int(item.Grabs))
	rel.Files = indexer.Ptr(int(item.NumFiles))
	rel.Seeders = indexer.Ptr(int(item.Seeders))
	rel.Peers = indexer.Ptr(int(item.Leechers + item.Seeders))
	if size, err := parse.Bytes(string(item.Size)); err == nil {
		rel.Size = indexer.Ptr(size)
	}
	factor := 1.0
	if free {
		factor = 0
	}
	rel.DownloadVolumeFactor = indexer.Ptr(factor)
	rel.UploadVolumeFactor = indexer.Ptr(1.0)
	rel.MinimumRatio = indexer.Ptr(1.0)
	rel.MinimumSeedTime = indexer.Ptr(int64(259200)) // 72 hours
	return rel, nil
}

// firstValues returns the first n values of a JSON object, in order and
// HTML-decoded.
func firstValues(object string, n int) ([]string, error) {
	if strings.TrimSpace(object) == "" || strings.TrimSpace(object) == "null" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(object)))
	if t, err := dec.Token(); err != nil {
		return nil, err
	} else if t != json.Delim('{') {
		return nil, fmt.Errorf("not an object: %s", object)
	}
	var values []string
	for dec.More() && len(values) < n {
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		var v jsonx.String
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		values = append(values, html.UnescapeString(string(v)))
	}
	return values, nil
}

// vipClasses are the user classes that get VIP torrents as freeleech.
var vipClasses = []string{"VIP", "Elite VIP"}

// hasUserVIP reports whether the user is of a VIP class, which it asks
// the site once an hour.
func (s *Site) hasUserVIP(ctx context.Context, ix *indexer.Indexer, cookies map[string]string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.After(s.classTill) {
		class, err := s.fetchUserClass(ctx, ix, cookies)
		if err != nil {
			return false, err
		}
		s.userClass, s.classTill = class, now.Add(userClassLife)
	}
	for _, c := range vipClasses {
		if strings.EqualFold(s.userClass, c) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Site) fetchUserClass(ctx context.Context, ix *indexer.Indexer, cookies map[string]string) (string, error) {
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/jsonLoad.php", indexer.AcceptJSON)
	r.Cookies = cookies
	slog.DebugContext(ctx, "fetching user data", "url", r.URL)
	resp, err := ix.Do(ctx, r)
	if err != nil {
		return "", fmt.Errorf("myanonamouse: fetching user data: %w", err)
	}
	if resp.HasError() {
		return "", &indexer.HTTPError{Response: resp}
	}
	var data struct {
		UserClass *string `json:"classname"`
	}
	if err := json.Unmarshal([]byte(resp.Content()), &data); err != nil {
		return "", fmt.Errorf("myanonamouse: user data: %w", err)
	}
	if data.UserClass == nil {
		return "", nil
	}
	return strings.TrimSpace(*data.UserClass), nil
}
