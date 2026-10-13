package hosszupuska

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "hosszupuska.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_episode":    "TestListCassette",
			"test_download_subtitle_episode": "TestDownloadCassette",
		},
	})
}

func episode() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "American Horror Story", Season: 10, Number: 1, Source: "Web",
		FileName: "American Horror Story s10e01 (amzn webrip-ntb).mkv",
	}
}

func cassetteProvider(t *testing.T, name string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: &http.Client{Transport: ported.Cassette(t, "test_hosszupuska/"+name)}, Now: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func TestListCassette(t *testing.T) {
	p := cassetteProvider(t, "test_list_subtitles_episode.yaml")
	subs, err := p.Search(t.Context(), episode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"http://hosszupuskasub.com/download.php?file=0124336.zip",
		"http://hosszupuskasub.com/download.php?file=0124335.zip",
		"http://hosszupuskasub.com/download.php?file=0124333.zip",
		"http://hosszupuskasub.com/download.php?file=0124253.zip",
	} {
		if !slices.ContainsFunc(subs, func(s provider.Subtitle) bool { return s.PageLink == want }) {
			t.Errorf("got = %+v, want = a subtitle of %s", subs, want)
		}
	}
	s := subs[0]
	if want := []string{"amzn webrip-ntb", "amzn web dl-ntb"}; !slices.Equal(s.ReleaseNames, want) || s.Language.Tag != "hu" {
		t.Errorf("got = %v in %q, want = %v in hu", s.ReleaseNames, s.Language.Tag, want)
	}
	if want := []string{"series", "season", "episode"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
}

func TestDownloadCassette(t *testing.T) {
	p := cassetteProvider(t, "test_download_subtitle_episode.yaml")
	v := episode()
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "hu"}})
	if err != nil || len(subs) == 0 {
		t.Fatalf("got = %v, %v, want = subtitles", subs, err)
	}
	f, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "srt" || len(f.Data) == 0 || f.Language.Tag != "hu" {
		t.Errorf("got = %d bytes of %q in %q, want = an srt in hu", len(f.Data), f.Format, f.Language.Tag)
	}
}

const row = `<tr bgcolor="#E9E9E9" onmouseout="this.style.backgroundImage='url(css/over2.jpg)';">
<td><img src="css/borito/x.png"></td>
<td><a href="#"><b>%NAME% s01e02</b></a><br>%TEXT%</td>
<td><img src='flags/%FLAG%.gif'></td><td>Kai</td><td>2021-09-01</td>
<td><table><tr><a href="#"><img src="css/infooldal.png"></a></tr></table></td>
<td><a href="https://adf.ly/1/%SERVER%download.php?file=%FILE%.zip">ad</a><a href="%SERVER%download.php?file=%FILE%.zip">dl</a></td>
</tr>`

func page(server string, rows ...[4]string) string {
	var b strings.Builder
	b.WriteString(`<html><body><table>`)
	for _, r := range rows {
		b.WriteString(strings.NewReplacer("%NAME%", r[0], "%TEXT%", r[1], "%FLAG%", r[2], "%FILE%", r[3], "%SERVER%", server).Replace(row))
	}
	b.WriteString(`</table></body></html>`)
	return b.String()
}

func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		w, err := zw.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[i+1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSearchAndDownload(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sorozatok.php":
			if r.Header.Get("User-Agent") != "Sub-Zero/2" || r.URL.Query().Get("cim") != "Mayans M.C." || r.URL.Query().Get("evad") != "01" {
				t.Errorf("got = %s with %q, want = the series' season", r.URL, r.Header.Get("User-Agent"))
			}
			server := srv.URL + "/"
			_, _ = w.Write([]byte(page(server,
				[4]string{"Mayans MC", "Mayans MC s01e02 (2018) (web-grp, 720p-LOL)", "1", "0001"},
				[4]string{"Mayans MC", "Mayans MC s01e02 s01e02 (hdtv-other)", "2", "0002"},
				[4]string{"Mayans MC", "Mayans MC s01e02 (hdtv-x)", "9", "0003"},
				[4]string{"Mayans MC", "no name", "1", "0004"},
			)))
		case "/download.php":
			switch r.URL.Query().Get("file") {
			case "0001.zip":
				_, _ = w.Write(zipOf(t, "Mayans.MC.S01E01.srt", "1\n00:00:01,000 --> 00:00:02,000\nA\n",
					"Mayans.MC.S01E02.srt", "1\r\n00:00:01,000 --> 00:00:02,000\r\nB\r\n"))
			case "0002.zip":
				_, _ = w.Write([]byte("<html>gone</html>"))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))
	defer srv.Close()
	p, _ := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)

	v := &provider.Video{Episode: true, Series: "Mayans M.C.", Season: 1, Number: 2, Year: 2018, ReleaseGroup: "DIMENSION"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "hu"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("got = %+v, want = 2 subtitles", subs)
	}
	if want := []string{"series", "season", "episode", "year", "release_group"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("matches got = %v, want = %v", subs[0].Matches, want)
	}
	if want := []string{"web-grp", "720p-LOL"}; !slices.Equal(subs[0].ReleaseNames, want) || subs[0].PageLink != srv.URL+"/download.php?file=0001.zip" {
		t.Errorf("got = %v at %s, want = %v at the site's link", subs[0].ReleaseNames, subs[0].PageLink, want)
	}
	if subs[1].Language.Tag != "en" || !slices.Equal(subs[1].ReleaseNames, []string{"hdtv-other"}) {
		t.Errorf("got = %+v, want = the English subtitle", subs[1])
	}
	f, err := p.Download(t.Context(), v, subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "1\n00:00:01,000 --> 00:00:02,000\nB\n" || f.Language.Tag != "hu" {
		t.Errorf("got = %q in %q, want = the episode's subtitle in hu", f.Data, f.Language.Tag)
	}
	if _, err := p.Download(t.Context(), v, subs[1].ID); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("got = %v, want = %v", err, provider.ProviderError)
	}
	id := url.Values{"u": {srv.URL + "/download.php?file=0009.zip"}}.Encode()
	if _, err := p.Download(t.Context(), v, id); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("got = %v, want = %v", err, provider.ServiceUnavailable)
	}
}
