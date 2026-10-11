package toloka

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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
	settings.Username, settings.Password = "user", "pass"
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
	return &indexer.Indexer{ID: "1", Name: "Toloka.to", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func html200(body string, cookies ...string) *http.Response {
	h := http.Header{"Content-Type": {"text/html; charset=utf-8"}}
	for _, c := range cookies {
		h.Add("Set-Cookie", c)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSettings(t *testing.T) {
	if _, err := Definition.New([]byte(`{}`)); err == nil {
		t.Error("New() without credentials error = nil, want an error")
	}
	caps := capabilities()
	if !caps.RawSearch {
		t.Error("RawSearch = false, want = true")
	}
	// 137 is mapped twice, as in Prowlarr.
	got := indexer.IDs(caps.Categories.MapTracker("137"))
	if !slices.Contains(got, indexer.MoviesDVD.ID) || !slices.Contains(got, indexer.TV.ID) {
		t.Errorf("categories of 137 = %v, want = Movies/DVD and TV", got)
	}
}

func TestSearchRequests(t *testing.T) {
	const prefix = "https://toloka.to/tracker.php?o=1&s=2&nm="
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"rss", Settings{}, indexer.Query{}, prefix},
		{"search", Settings{}, indexer.Query{Term: "Spider-Man"}, prefix + "Spider+Man"},
		{
			"tv season",
			Settings{},
			indexer.Query{Type: indexer.SearchTV, Term: "Friends", Season: indexer.Ptr(2), Episode: "3"},
			prefix + "Friends+%D0%A1%D0%B5%D0%B7%D0%BE%D0%BD+2",
		},
		{"tv season 0", Settings{}, indexer.Query{Type: indexer.SearchTV, Term: "Friends", Season: indexer.Ptr(0)}, prefix + "Friends"},
		{
			"freeleech and categories",
			Settings{FreeleechOnly: true},
			indexer.Query{Type: indexer.SearchMovie, Term: "Dune", Categories: []int{indexer.TVAnime.ID}},
			prefix + "Dune&sds=1&f[]=127&f[]=194",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
			if got := c.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestParseTitle(t *testing.T) {
	tv := []int{indexer.TV.ID}
	movies := []int{indexer.Movies.ID}
	tests := []struct {
		title string
		cats  []int
		strip bool
		want  string
	}{
		{
			"Друзі (Сезон 1, серії 1-24 з 24) / Friends (Season 1, episodes 1-24 of 24) (1994) WEB-DLRip 1080p Ukr/Eng | sub Eng", tv, true,
			// Only the first tag leading the title moves.
			"Friends (S1E1-24 of 24) (1994) WEB-DL 1080p Ukr/Eng | sub Eng (S1E1-24 of 24)",
		},
		{
			"Друзі (Сезон 1, серії 1-24 з 24) / Friends (Season 1, episodes 1-24 of 24) (1994) WEB-DLRip 1080p Ukr/Eng | sub Eng", tv, false,
			"Друзі (S1E1-24 of 24) / Friends (S1E1-24 of 24) (1994) WEB-DL 1080p Ukr/Eng | sub Eng",
		},
		{
			"Відьмак (Сезон 3, серії 1-8 з Х) / The Witcher (Season 3, episodes 1-8 of X) (2023) WEB-DL 1080p", tv, true,
			"The Witcher (S3E1-8 of X) (2023) WEB-DL 1080p (S3E1-8 of XX)",
		},
		{"[Mobile] Дюна: Частина друга / Dune: Part Two (2024) WEBRip 720p", movies, true, "Dune: Part Two (2024) WEBRip 720p [Mobile]"},
		{"Шрек – Shrek (2001) HDTVRip", movies, true, "Shrek (2001) HDTV"},
		// Seasons are not rewritten outside TV.
		{"Concert (Season 2) WEBDL", movies, false, "Concert (Season 2) WEB-DL"},
	}
	for _, tt := range tests {
		if got := parseTitle(tt.title, tt.cats, tt.strip); got != tt.want {
			t.Errorf("parseTitle(%q) = %q, want = %q", tt.title, got, tt.want)
		}
	}
}

const page = `<html><body><a href="login.php?logout=true">Вихід</a>
<table class="forumline"><tbody>
<tr><th>header</th></tr>
<tr class="prow1">
 <td></td>
 <td><a href="tracker.php?f=124">Телесеріали</a></td>
 <td><a href="t123456">Друзі (Сезон 1, серії 1-24 з 24) / Friends (Season 1, episodes 1-24 of 24) (1994) WEB-DLRip 1080p</a></td>
 <td></td>
 <td><img src="images/silver.gif"></td>
 <td><a href="download.php?id=987">DL</a></td>
 <td>45.3 GB</td>
 <td></td>
 <td>1,204</td>
 <td><b>12</b></td>
 <td><b> 3 </b></td>
 <td></td>
 <td>2023-01-21</td>
</tr>
<tr class="prow2">
 <td></td>
 <td><a href="tracker.php?f=96">Фільми в HD</a></td>
 <td><a href="t2">Дюна / Dune (2021) BDRip 1080p</a></td>
 <td></td>
 <td><img src="images/gold.gif"></td>
 <td><a href="download.php?id=2">DL</a></td>
 <td>8 GB</td>
 <td></td>
 <td>5</td>
 <td><b>1</b></td>
 <td><b>0</b></td>
 <td></td>
 <td>2023-02-01</td>
</tr>
<tr class="prow1">
 <td></td><td><a href="tracker.php?f=96">x</a></td><td><a href="t3">Awaiting moderation</a></td>
 <td></td><td></td><td>moderation</td>
</tr>
</tbody></table></body></html>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, Settings{}, func(*http.Request) *http.Response { return html200(page) })
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	tv, movie := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", tv.Title, "Friends (S1E1-24 of 24) (1994) WEB-DL 1080p (S1E1-24 of 24)"},
		{"Description", tv.Description, "Друзі (Сезон 1, серії 1-24 з 24) / Friends (Season 1, episodes 1-24 of 24) (1994) WEB-DLRip 1080p"},
		{"GUID", tv.GUID, "https://toloka.to/t123456"},
		{"InfoURL", tv.InfoURL, "https://toloka.to/t123456"},
		{"DownloadURL", tv.DownloadURL, "https://toloka.to/download.php?id=987"},
		{"Category", tv.Categories[0], indexer.TV.ID},
		{"Seeders", *tv.Seeders, 12},
		{"Peers", *tv.Peers, 15},
		{"Grabs", *tv.Grabs, 1204},
		{"Size", *tv.Size, int64(48640504627)},
		{"PublishDate", tv.PublishDate.Format(time.DateOnly), "2023-01-21"},
		{"DownloadVolumeFactor", *tv.DownloadVolumeFactor, 0.5},
		{"UploadVolumeFactor", *tv.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *tv.MinimumRatio, 1.0},
		{"MinimumSeedTime", *tv.MinimumSeedTime, int64(0)},
		{"movie Title", movie.Title, "Dune (2021) BDRip 1080p"},
		{"movie Category", movie.Categories[0], indexer.MoviesHD.ID},
		{"movie DownloadVolumeFactor", *movie.DownloadVolumeFactor, 0.0},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name, answer, wantErr string
	}{
		{"success", `<a href="login.php?logout=true">Вихід</a>`, ""},
		{"error", `<table class="forumline"><tr><td><table><tr><td><span class="gen">Невірне ім'я або пароль<br><a href="x">Назад</a></span></td></tr></table></td></tr></table>`, "Невірне ім'я або пароль"},
		{"unknown", `<html></html>`, "Unknown error message, please report."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var form url.Values
				ix := newIndexer(t, Settings{}, func(r *http.Request) *http.Response {
					if r.Method != http.MethodPost || r.URL.String() != "https://toloka.to/login.php" {
						t.Errorf("login = %s %s", r.Method, r.URL)
					}
					if got := r.Header.Get("Referer"); got != "https://toloka.to/login.php" {
						t.Errorf("Referer = %q", got)
					}
					b, _ := io.ReadAll(r.Body)
					form, _ = url.ParseQuery(string(b))
					return html200(tt.answer, "toloka_sid=s1; Path=/")
				})
				err := ix.Site.(indexer.LoginSite).Login(context.Background(), ix)
				want := url.Values{
					"username": {"user"}, "password": {"pass"}, "autologin": {"on"},
					"ssl": {"on"}, "redirect": {""}, "login": {"Вхід"},
				}
				if form.Encode() != want.Encode() {
					t.Errorf("form = %v, want = %v", form, want)
				}
				if tt.wantErr == "" {
					if err != nil {
						t.Fatal(err)
					}
					if got := ix.Cookies()["toloka_sid"]; got != "s1" {
						t.Errorf("cookie toloka_sid = %q, want = s1", got)
					}
					return
				}
				if !errors.Is(err, indexer.ErrAuth) || !strings.HasSuffix(err.Error(), tt.wantErr) {
					t.Errorf("Login() error = %v, want = %q", err, tt.wantErr)
				}
			})
		})
	}
}
