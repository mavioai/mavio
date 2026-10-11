package rutracker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

type handler func(r *http.Request) *http.Response

func (h handler) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := h(r)
	resp.Request = r
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp, nil
}

func newIndexer(t *testing.T, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := Definition.New(data)
	if err != nil {
		t.Fatal(err)
	}
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = h
	return &indexer.Indexer{ID: "1", Name: "RuTracker", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "Юзер", Password: "p@ss word"}}

// html1251 returns a page as RuTracker sends it, in Windows-1251.
func html1251(t *testing.T, status int, header http.Header, page string) *http.Response {
	t.Helper()
	body, err := charmap.Windows1251.NewEncoder().String(page)
	if err != nil {
		t.Fatal(err)
	}
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "text/html; charset=windows-1251")
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSearchRequests(t *testing.T) {
	const base = "https://rutracker.org/forum/tracker.php?nm="
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{"rss", indexer.Query{}, base},
		{"blank", indexer.Query{Term: "  "}, base + "++"},
		{"wildcards", indexer.Query{Term: "Spider-Man: No Way Home"}, base + "Spider%25Man%25No%25Way%25Home"},
		{"cyrillic", indexer.Query{Term: "Ёж (2020)"}, base + "%D0%81%D0%B6%252020%25"},
		{
			"tv season",
			indexer.Query{Type: indexer.SearchTV, Term: "Friends", Season: indexer.Ptr(2), Episode: "3"},
			base + "Friends+%D0%A2%D0%92+%7C+%D0%A1%D0%B5%D0%B7%D0%BE%D0%BD%3A+2",
		},
		{"tv without season", indexer.Query{Type: indexer.SearchTV, Term: "Friends"}, base + "Friends"},
		{"forum", indexer.Query{Term: "x", Categories: []int{indexer.PCMobileiOS.ID}}, base + "x&f=1933%2C1935%2C1003%2C1937%2C2235%2C1908%2C864%2C863%2C2535%2C2534"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if r.FollowRedirects {
				t.Errorf("FollowRedirects = true, want = false")
			}
		})
	}
}

func TestSearchRequestsChunkForums(t *testing.T) {
	ix := newIndexer(t, testSettings, nil)
	site := ix.Site.(*Site)
	q := &indexer.Query{Term: "x", Categories: []int{indexer.Movies.ID, indexer.TV.ID}}
	forums := site.caps.Categories.MapToTracker(q.Categories, false)
	c, err := site.SearchRequests(context.Background(), ix, q)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, pages := range c.Tiers()[0] {
		for r := range pages {
			_, f, _ := strings.Cut(r.URL, "&f=")
			chunk := strings.Split(strings.ReplaceAll(f, "%2C", ","), ",")
			if len(chunk) > maxCategories {
				t.Errorf("forums in a search = %d, want = at most %d", len(chunk), maxCategories)
			}
			got = append(got, chunk...)
		}
	}
	if want := (len(forums) + maxCategories - 1) / maxCategories; len(c.Tiers()[0]) != want || want < 2 {
		t.Errorf("searches = %d, want = %d (more than 1)", len(c.Tiers()[0]), want)
	}
	if !slices.Equal(got, forums) {
		t.Errorf("forums = %v, want = %v", got, forums)
	}
}

const page = `<html><body>
<a id="logged-in-username" href="profile.php">Юзер</a>
<table id="tor-tbl"><tbody>
<tr id="trs-tr-1">
 <td class="row1 t-ico"></td>
 <td class="row1 t-ico"></td>
 <td class="row1 f-name-col"><div class="f-name"><a href="tracker.php?f=189">Зарубежные сериалы</a></div></td>
 <td class="row4 med tLeft t-title-col tt"><div class="t-title"><a class="tLink" href="viewtopic.php?t=100"> Друзья / Friends / Сезон: 1 / Серии: 1-24 из 24 (Дэвид Крейн) [1994, США, комедия, WEB-DLRip] MVO (Paramount) </a></div></td>
 <td class="row1 u-name-col"></td>
 <td class="row4 small nowrap tor-size" data-ts_text="1610612736"><a class="small tr-dl dl-stub" href="dl.php?t=100">1.5 GB</a></td>
 <td class="row4 nowrap" data-ts_text="12"><b class="seedmed">12</b></td>
 <td class="row4 leechmed bold">3</td>
 <td class="row4 small number-format">1 234</td>
 <td class="row4 small nowrap" data-ts_text="1600000000"><p>13-Сен-20</p></td>
</tr>
<tr id="trs-tr-2">
 <td class="row1 t-ico"></td>
 <td class="row1 t-ico"></td>
 <td class="row1 f-name-col"><div class="f-name"><a href="tracker.php?f=22">Наше кино</a></div></td>
 <td class="row4 med tLeft t-title-col tt"><div class="t-title"><a class="tLink" href="viewtopic.php?t=200">Брат (Алексей Балабанов) [1997, Россия, драма, BDRip 1080p]</a></div></td>
 <td class="row1 u-name-col"></td>
 <td class="row4 small nowrap tor-size" data-ts_text="2048"><a class="small tr-dl dl-stub" href="dl.php?t=200">2 KB</a></td>
 <td class="row4 nowrap" data-ts_text="-5" title="Последний раз был 5 дн назад"><span class="seedmed">5 дн</span></td>
 <td class="row4 leechmed bold">0</td>
 <td class="row4 small number-format">7</td>
 <td class="row4 small nowrap" data-ts_text="1700000000"><p>14-Ноя-23</p></td>
</tr>
<tr id="trs-tr-3">
 <td class="row1 t-ico"></td>
 <td class="row1 t-ico"></td>
 <td class="row1 f-name-col"><div class="f-name"><a href="tracker.php?f=22">Наше кино</a></div></td>
 <td class="row4 med tLeft t-title-col tt"><div class="t-title"><a class="tLink" href="viewtopic.php?t=300">На модерации</a></div></td>
 <td class="row1 u-name-col"></td>
 <td class="row4 small nowrap tor-size" data-ts_text="0">—</td>
</tr>
</tbody></table></body></html>`

func TestParse(t *testing.T) {
	for _, magnet := range []bool{false, true} {
		s := testSettings
		s.RussianLetters = true
		s.UseMagnetLinks = magnet
		ix := newIndexer(t, s, func(*http.Request) *http.Response { return html1251(t, http.StatusOK, nil, page) })
		releases, err := ix.Search(context.Background(), &indexer.Query{Term: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if len(releases) != 2 {
			t.Fatalf("releases = %d, want = 2", len(releases))
		}
		movie, show := releases[0], releases[1]
		download := "https://rutracker.org/forum/dl.php?t=100"
		if magnet {
			download = "https://rutracker.org/forum/viewtopic.php?t=100"
		}
		for _, c := range []struct {
			name      string
			got, want any
		}{
			{"Title", show.Title, "Friends / S1E1-24 of 24 [1994, WEB-DL]"},
			{"Description", show.Description, "Друзья / Friends / Сезон: 1 / Серии: 1-24 из 24 (Дэвид Крейн) [1994, США, комедия, WEB-DLRip] MVO (Paramount)"},
			{"GUID", show.GUID, "https://rutracker.org/forum/viewtopic.php?t=100"},
			{"InfoURL", show.InfoURL, "https://rutracker.org/forum/viewtopic.php?t=100"},
			{"DownloadURL", show.DownloadURL, download},
			{"Categories", slices.Equal(show.Categories, []int{indexer.TVForeign.ID, 100189}), true},
			{"Size", *show.Size, int64(1610612736)},
			{"Seeders", *show.Seeders, 12},
			{"Peers", *show.Peers, 15},
			{"Grabs", *show.Grabs, 1234},
			{"PublishDate", show.PublishDate, time.Unix(1600000000, 0).UTC()},
			{"DownloadVolumeFactor", *show.DownloadVolumeFactor, 1.0},
			{"UploadVolumeFactor", *show.UploadVolumeFactor, 1.0},
			{"MinimumRatio", *show.MinimumRatio, 1.0},
			{"MinimumSeedTime", *show.MinimumSeedTime, int64(0)},
			{"movie Title", movie.Title, "[1997, BDRip 1080p]"},
			{"movie Categories", slices.Equal(movie.Categories, []int{indexer.Movies.ID, 100022}), true},
			{"movie Seeders", *movie.Seeders, 0},
			{"movie Peers", *movie.Peers, 0},
			{"movie Grabs", *movie.Grabs, 7},
		} {
			if c.got != c.want {
				t.Errorf("magnet %v: %s = %v, want = %v", magnet, c.name, c.got, c.want)
			}
		}
	}
}

func TestLoginNeeded(t *testing.T) {
	site := &Site{}
	tests := []struct {
		name   string
		header http.Header
		body   string
		want   bool
	}{
		{"logged in", nil, `<a id="logged-in-username">u</a>`, false},
		{"logged out", nil, `<form action="login.php">`, true},
		{"redirect", http.Header{"Location": {"https://rutracker.org/forum/login.php?redirect=tracker.php"}}, `<a id="logged-in-username">u</a>`, true},
		{"refresh", http.Header{"Refresh": {"0; url=login.php"}}, `<a id="logged-in-username">u</a>`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.header
			if h == nil {
				h = http.Header{}
			}
			r := &indexer.Response{StatusCode: http.StatusOK, Header: h, Body: []byte(tt.body)}
			if got := site.LoginNeeded(r); got != tt.want {
				t.Errorf("LoginNeeded() = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	const wantForm = "login_username=%DE%E7%E5%F0&login_password=p%40ss+word&login=Login&redirect=index.php"
	tests := []struct {
		name    string
		page    string
		wantErr string
	}{
		{"success", page, ""},
		{"wrong password", `<h4 class="warnColor1 tCenter mrg_16"> неверный пароль </h4>`, "неверный пароль"},
		{"message", `<div class="msg-main">Доступ закрыт</div>`, "Доступ закрыт"},
		{"unknown", `<html></html>`, "RuTracker Auth Failed"},
		{"captcha", `<input type="hidden" name="cap_sid" value="abc"><input name="cap_code_123">`, "captcha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var searches int
				ix := newIndexer(t, testSettings, func(r *http.Request) *http.Response {
					if strings.HasSuffix(r.URL.Path, "/forum/login.php") {
						body, _ := io.ReadAll(r.Body)
						for _, c := range []struct{ name, got, want string }{
							{"method", r.Method, http.MethodPost},
							{"form", string(body), wantForm},
							{"Referer", r.Header.Get("Referer"), "https://rutracker.org/forum/login.php"},
							{"Content-Type", r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"},
							{"Cookie", r.Header.Get("Cookie"), ""},
						} {
							if c.got != c.want {
								t.Errorf("login %s = %q, want = %q", c.name, c.got, c.want)
							}
						}
						return html1251(t, http.StatusOK, http.Header{"Set-Cookie": {"bb_session=secret; Path=/"}}, tt.page)
					}
					searches++
					if searches == 1 {
						return html1251(t, http.StatusFound, http.Header{"Location": {"login.php?redirect=tracker.php"}}, "")
					}
					if got := r.Header.Get("Cookie"); got != "bb_session=secret" {
						t.Errorf("search Cookie = %q, want = %q", got, "bb_session=secret")
					}
					return html1251(t, http.StatusOK, nil, page)
				})
				releases, err := ix.Search(context.Background(), &indexer.Query{Term: "x"})
				if tt.wantErr == "" {
					if err != nil || len(releases) != 2 {
						t.Fatalf("Search() = %d releases, %v, want = 2 releases, nil", len(releases), err)
					}
					if got := ix.Cookies()["bb_session"]; got != "secret" {
						t.Errorf("cookie bb_session = %q, want = %q", got, "secret")
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !errors.Is(err, indexer.ErrAuth) {
					t.Errorf("Search() error = %v, want = an auth error containing %q", err, tt.wantErr)
				}
			})
		})
	}
}

func TestDownload(t *testing.T) {
	const magnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=x"
	tests := []struct {
		name    string
		magnet  bool
		link    string
		topic   string
		want    string
		wantErr string
	}{
		{"torrent", false, "https://rutracker.org/forum/dl.php?t=100", "", "torrent", ""},
		{
			"magnet", true, "https://rutracker.org/forum/viewtopic.php?t=100",
			`<table class="attach"><tr><td><a class="magnet-link" href="` + magnet + `">magnet</a></td></tr></table>`, magnet, "",
		},
		{"no magnet", true, "https://rutracker.org/forum/viewtopic.php?t=100", `<html></html>`, "", "Failed to fetch magnet link from https://rutracker.org/forum/viewtopic.php?t=100"},
		{"magnet setting, torrent link", true, "https://rutracker.org/forum/dl.php?t=100", "", "torrent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testSettings
			s.UseMagnetLinks = tt.magnet
			ix := newIndexer(t, s, func(r *http.Request) *http.Response {
				if got := r.Header.Get("Cookie"); got != "bb_session=secret" {
					t.Errorf("Cookie = %q, want = %q", got, "bb_session=secret")
				}
				if strings.Contains(r.URL.String(), "viewtopic.php") {
					return html1251(t, http.StatusOK, nil, tt.topic)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-bittorrent"}}, Body: io.NopCloser(strings.NewReader("torrent"))}
			})
			ix.SetCookies(map[string]string{"bb_session": "secret"}, time.Time{})
			data, err := ix.Download(context.Background(), tt.link)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Download() error = %v, want = containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || string(data) != tt.want {
				t.Errorf("Download() = %q, %v, want = %q, nil", data, err, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{"username":"u"}`)); err == nil || !strings.Contains(err.Error(), "password") {
		t.Errorf("New() error = %v, want = a password error", err)
	}
}

func TestParseTitle(t *testing.T) {
	tv := []int{indexer.TV.ID, 100189}
	anime := []int{indexer.TVAnime.ID}
	movie := []int{indexer.Movies.ID}
	uhd := []int{indexer.MoviesUHD.ID}
	other := []int{indexer.PC.ID}
	strip := TitleOptions{StripCyrillic: true}
	const irishman = "Ирландец / The Irishman (Мартин Скорсезе / Martin Scorsese) [2019, США, криминал, драма, биография, WEB-DL 1080p] Dub (Пифагор) + MVO (Jaskier) + AVO (Юрий Сербин) + Sub Rus, Eng + Original Eng"
	tests := []struct {
		name  string
		title string
		cats  []int
		o     TitleOptions
		want  string
	}{
		{"movie", irishman, movie, TitleOptions{}, "Ирландец / The Irishman [2019, США, криминал, драма, биография, WEB-DL 1080p] Dub + AVO + Original Eng"},
		{"movie stripped", irishman, movie, strip, "The Irishman [2019, WEB-DL 1080p] Dub + AVO + Original Eng"},
		{
			"blu-ray disc and RUS",
			"Терминатор / The Terminator (Джеймс Кэмерон / James Cameron) [1984, США, Blu-ray Disc 1080p] [Remux]",
			uhd,
			TitleOptions{StripCyrillic: true, AddRussianToTitle: true},
			"The Terminator [1984, BR-DISK 1080p] [Remux] RUS",
		},
		{"RUS only for movies and TV", "Program [2020, x64]", other, TitleOptions{AddRussianToTitle: true}, "Program [2020, x64]"},
		{
			"season and episodes of",
			"Друзья / Friends / Сезон: 1 / Серии: 1-24 из 24 (Дэвид Крейн) [1994, США, комедия, WEB-DLRip] MVO (Paramount)",
			tv, strip, "Friends / S1E1-24 of 24 [1994, WEB-DL]",
		},
		{
			"season and episodes",
			"Сверхъестественное / Supernatural / Сезон: 3 / Серии: 1-5 (16) [2007, США, HDTVRip] 2x MVO (Novafilm)",
			tv, strip, "Supernatural / S3E1-5 (16) [2007, HDTV]",
		},
		{"season", "Шерлок / Sherlock / Сезон: 1-4 [2010, WEBDL 720p]", tv, strip, "Sherlock / S1-4 [2010, WEB-DL 720p]"},
		{"episodes", "Шоу / Show / Выпуски: 5 из ? [2021, WEBRip]", tv, strip, "Show / E5 of ? [2021, WEBRip]"},
		{
			"anime",
			"Атака титанов / Shingeki no Kyojin (ТВ-2) [1-12 из 12] [2017, TVRip 2160p, 4K]",
			anime, strip, "Shingeki no Kyojin (S2) [1-12 12] [2017, TVRip 2160p]",
		},
		{"anime episodes", "Ван-Пис / One Piece (ТВ) [12+1 из 24] [2020, HDTV]", anime, strip, "One Piece E12+1 of 24 [2020, HDTV]"},
		{"comma episodes", "Show / Сезон: 2 / Серии: 1,10 [2020]", tv, TitleOptions{}, "Show / S2E1-10 [2020]"},
		{"cyrillic X", "Сериал - Х - Show [2020, ХХ]", tv, TitleOptions{}, "Сериал - XX - Show [2020, XX]"},
		{"dashes", "Artist – Album — 2020", []int{indexer.Audio.ID}, TitleOptions{}, "Artist - Album - 2020"},
		{"R5", "Movie (2009) R5 [DVDRip]", movie, TitleOptions{}, "Movie (2009)"},
		{"kurazh", "Кураж-Бамбей WEB-DLRip", movie, TitleOptions{}, "kurazh WEB-DL"},
		{"empty brackets", "Title ( / English) [ ] +", other, TitleOptions{}, "Title (English)"},
		{"move first tags", "[Ubuntu] [x64] Some Linux distro (2020) [ISO]", other, TitleOptions{MoveFirstTags: true}, "[x64] Some Linux distro (2020) [ISO] [Ubuntu]"},
		{"move first tags of both kinds", "(Rock) [CD] Artist – Album – 2020, FLAC", other, TitleOptions{MoveFirstTags: true}, "Artist - Album - 2020, FLAC (Rock) [CD]"},
		{"move all tags", "[Ubuntu] [x64] Some Linux distro (2020) [ISO]", other, TitleOptions{MoveAllTags: true}, "Some Linux distro (2020) [Ubuntu] [x64] [ISO]"},
		{
			"move all tags wins",
			"[Ubuntu] (nested (tag)) Distro", other,
			TitleOptions{MoveAllTags: true, MoveFirstTags: true},
			"Distro (nested (tag)) [Ubuntu]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseTitle(tt.title, tt.cats, tt.o); got != tt.want {
				t.Errorf("ParseTitle() = %q, want = %q", got, tt.want)
			}
		})
	}
}
