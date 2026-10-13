package zimuku

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// bmp24 makes a top-down 24-bit BMP of a red and a blue pixel.
func bmp24() []byte {
	b := make([]byte, 54+8)
	copy(b, "BM")
	le := binary.LittleEndian
	le.PutUint32(b[2:], uint32(len(b)))
	le.PutUint32(b[10:], 54)
	le.PutUint32(b[14:], 40)
	le.PutUint32(b[18:], 2)
	le.PutUint32(b[22:], uint32(0xffffffff)) // -1: top-down.
	le.PutUint16(b[26:], 1)
	le.PutUint16(b[28:], 24)
	copy(b[54:], []byte{0, 0, 0xff, 0xff, 0, 0})
	return b
}

// bmp8 makes a bottom-up 8-bit BMP of one column, white over black.
func bmp8() []byte {
	b := make([]byte, 54+8+8)
	copy(b, "BM")
	le := binary.LittleEndian
	le.PutUint32(b[10:], 62)
	le.PutUint32(b[14:], 40)
	le.PutUint32(b[18:], 1)
	le.PutUint32(b[22:], 2)
	le.PutUint16(b[26:], 1)
	le.PutUint16(b[28:], 8)
	le.PutUint32(b[46:], 2)
	copy(b[54:], []byte{0, 0, 0, 0, 0xff, 0xff, 0xff, 0})
	b[62], b[66] = 0, 1
	return b
}

// solver reads the fake site's captcha.
type solver struct {
	images int
	fail   bool
}

func (s *solver) ReCaptchaV2(context.Context, string, string, bool) (string, error) {
	return "", errors.New("no reCAPTCHA")
}

func (s *solver) Image(_ context.Context, image []byte) (string, error) {
	s.images++
	img, err := png.Decode(bytes.NewReader(image))
	if err != nil {
		return "", err
	}
	if s.fail || color.RGBAModel.Convert(img.At(0, 0)) != (color.RGBA{R: 0xff, A: 0xff}) {
		return "", errors.New("unreadable")
	}
	return "1234", nil
}

// fakeSite is Zimuku behind its verification, asking for the captcha
// "1234" on searches and random coordinates elsewhere.
type fakeSite struct {
	srv       *httptest.Server
	verifies  []string
	downloads int
}

func verified(r *http.Request) bool {
	c, err := r.Cookie("security_session_verify")
	return err == nil && c.Value == "ok"
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	var z bytes.Buffer
	w := zip.NewWriter(&z)
	for _, n := range []string{".hidden.chs&eng.srt", "Show.S01E01.eng.srt", "Show.S01E01.chs&eng.srt", "readme.nfo"} {
		fw, _ := w.Create(n)
		_, _ = fmt.Fprintf(fw, "1\r\n00:00:01,000 --> 00:00:02,000\r\n%s\r\n", n)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	guard := func(captcha bool, h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !verified(r) {
				w.WriteHeader(http.StatusNotFound)
				img := ""
				if captcha {
					img = `<img class="c" src="data:image/bmp;base64,` + base64.StdEncoding.EncodeToString(bmp24()) + `" alt="">`
				}
				_, _ = fmt.Fprintf(w, `<html>%s<script>self.location = "/verify?code=" + stringToHex(code);</script></html>`, img)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		f.verifies = append(f.verifies, code)
		if c, err := r.Cookie("srcurl"); err != nil || !strings.HasPrefix(c.Value, "687474703a2f2f") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if code == "31323334" || strings.Contains(code, "2c") {
			http.SetCookie(w, &http.Cookie{Name: "security_session_verify", Value: "ok", Path: "/"})
		}
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	})
	mux.HandleFunc("GET /search", guard(true, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("q") {
		case "The Show.S01", "The Show.S11":
			_, _ = fmt.Fprint(w, `<script>var url = ''; url = '?id=1' + url; url = '/result' + url; location = url;</script>`)
		case "Movie 2010":
			_, _ = fmt.Fprint(w, `<div class="item"><p class="tt clearfix"><a href="/detail/9.html">电影 Movie (2010)</a></p></div>`)
		}
	}))
	mux.HandleFunc("GET /result", guard(false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<div class="box">
			<div class="item"><p class="tt clearfix"><a href="/detail/1.html">节目 第一季 The Show Season 1 (2011)</a></p></div>
			<div class="item"><p class="tt clearfix"><a href="/detail/2.html">节目 第二季 The Show Season 2 (2012)</a></p></div>
			<div class="item"><p class="tt clearfix"><a href="/detail/3.html">节目 第十一季 (2021)</a></p></div></div>`)
	}))
	mux.HandleFunc("GET /detail/{id}", guard(false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<table><tbody>
			<tr><td><a href="/detail/sub/10.html">节目.The.Show.S01E01.WEB.简繁英双语.zip</a></td><td class="tac lang"><img src="/flag/china.gif"><img src="/flag/hongkong.gif"></td></tr>
			<tr><td><a href="/detail/sub/11.html">The.Show.S01E02.HDTV 简体</a></td><td class="tac lang"><img src="/flag/jollyroger.gif"></td></tr>
			<tr><td><a href="/detail/sub/12.html">节目第三集</a></td><td class="tac lang"><img src="/flag/hongkong.gif"><img src="/flag/uk.gif"></td></tr>
			<tr><td><a href="/detail/sub/13.html">The.Show.S01E04</a></td><td class="tac lang"><img src="/flag/uk.gif"></td></tr>
			</tbody></table>`)
	}))
	mux.HandleFunc("GET /detail/sub/{id}", guard(false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<a id="down1" href="/dld/%s">下载</a>`, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /dld/{id}", guard(false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<a rel="nofollow" href="../../download/%s">电信</a>`, r.PathValue("id"))
	}))
	mux.HandleFunc("GET /download/{id}", guard(false, func(w http.ResponseWriter, r *http.Request) {
		f.downloads++
		id := strings.TrimSuffix(r.PathValue("id"), ".html")
		if !strings.HasSuffix(r.Referer(), "/detail/sub/"+id+".html") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch id {
		case "10":
			w.Header().Set("Content-Disposition", `attachment; filename="Show.S01E01.ZIP"`)
			_, _ = w.Write(z.Bytes())
		case "11":
			w.Header().Set("Content-Disposition", `attachment; filename="Show.S01E02.srt"`)
			_, _ = fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nHi\n")
		case "12":
			w.Header().Set("Content-Disposition", `attachment; filename="Show.S01E03.rar"`)
			_, _ = w.Write(z.Bytes())
		}
	}))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite, s provider.CaptchaSolver) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler), Captcha: s}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	s := &solver{}
	p := newProvider(t, f, s)
	v := &provider.Video{Episode: true, Series: "The Show", Season: 1, Number: 1, Year: 2011}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "zh"}, {Tag: "zh-Hant"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, sub := range subs {
		got = append(got, fmt.Sprintf("%s %s %q %v", strings.TrimPrefix(sub.ID, f.srv.URL), sub.Language.Tag, sub.ReleaseNames, sub.Matches))
	}
	want := []string{
		`/detail/sub/10.html zh ["The.Show.S01E01.WEB."] [year]`,
		`/detail/sub/10.html zh-Hant ["The.Show.S01E01.WEB."] [year]`,
		`/detail/sub/11.html zh ["The.Show.S01E02"] [year]`,
		`/detail/sub/12.html zh-Hant [] [year]`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}
	if s.images != 1 || len(f.verifies) != 1 || f.verifies[0] != "31323334" {
		t.Errorf("verifications got = %d, %q, want one of the captcha", s.images, f.verifies)
	}

	// A movie is searched with its year; the page's year is the video's.
	movie := &provider.Video{Title: "Movie", Year: 2010}
	if subs, err := p.Search(t.Context(), movie, []provider.Language{{Tag: "zh-Hant"}}); err != nil || len(subs) != 2 || !slices.Equal(subs[0].Matches, []string{"year"}) {
		t.Errorf("movie got = %+v, %v", subs, err)
	}
	// The year of the eleventh season's page is the show's.
	if subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "The Show", Season: 11, Number: 1, Year: 2011}, []provider.Language{{Tag: "zh"}}); err != nil || len(subs) != 2 || !slices.Equal(subs[1].Matches, []string{"year"}) {
		t.Errorf("eleventh season got = %+v, %v", subs, err)
	}
	// Seasons not found on the site.
	if subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "The Show", Season: 3, Number: 1}, []provider.Language{{Tag: "zh"}}); err != nil || len(subs) != 0 {
		t.Errorf("unknown season got = %+v, %v, want none", subs, err)
	}
}

func TestCaptcha(t *testing.T) {
	f := newFake(t)
	v := &provider.Video{Title: "Movie", Year: 2010}
	if _, err := newProvider(t, f, nil).Search(t.Context(), v, []provider.Language{{Tag: "zh"}}); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("no solver got = %v, want = ConfigurationError", err)
	}
	if _, err := newProvider(t, f, &solver{fail: true}).Search(t.Context(), v, []provider.Language{{Tag: "zh"}}); err == nil {
		t.Errorf("unsolved captcha got = nil, want an error")
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, &solver{})
	file, err := p.Download(t.Context(), nil, f.srv.URL+"/detail/sub/10.html")
	if want := "1\n00:00:01,000 --> 00:00:02,000\nShow.S01E01.chs&eng.srt\n"; err != nil || string(file.Data) != want || file.Format != "srt" {
		t.Errorf("archive got = %q, %v, want = %q", file.Data, err, want)
	}
	// Random coordinates pass the verification of pages without captcha.
	if len(f.verifies) != 1 || !strings.Contains(f.verifies[0], "2c") {
		t.Errorf("verifications got = %q, want coordinates", f.verifies)
	}
	if file, err := p.Download(t.Context(), nil, f.srv.URL+"/detail/sub/11.html"); err != nil || string(file.Data) != "1\n00:00:01,000 --> 00:00:02,000\nHi\n" {
		t.Errorf("subtitle got = %q, %v", file.Data, err)
	}
	// A zip named a RAR is dropped, as Bazarr's.
	if _, err := p.Download(t.Context(), nil, f.srv.URL+"/detail/sub/12.html"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("misnamed archive got = %v, want = BadSubtitle", err)
	}
	if _, err := p.Download(t.Context(), nil, f.srv.URL+"/detail/sub/13.html"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("no file got = %v, want = BadSubtitle", err)
	}
}

func TestExtractName(t *testing.T) {
	// The names Bazarr's _extract_name gives.
	for name, want := range map[string]string{
		"权力的游戏.Game.of.Thrones.S01E01.chs.ass": "Game.of.Thrones.S01E01.chs.ass",
		"Game.of.Thrones.S01E01.权力的游戏.zip":     "Game.of.Thrones.S01E01..zip",
		"权力的游戏 第一季":                            "",
		"中文A.B中文Longer Part中C":                 "A.B中文Longer Part中C",
		"The Show 第1集 简体 Extra.srt":            "The Show.srt",
		".hidden":                              "hidden",
		"a中bc中d":                               "bc",
		"Show.S01E02.WEB 简英双语":                 "Show.S01E02.WEB 简英双语",
	} {
		if got := extractName(name); got != want {
			t.Errorf("extractName(%q) got = %q, want = %q", name, got, want)
		}
	}
}

func TestNumToCN(t *testing.T) {
	for n, want := range map[int]string{1: "一", 9: "九", 10: "十", 11: "十一", 20: "二十", 99: "九十九", 0: "", 100: ""} {
		if got := numToCN(n); got != want {
			t.Errorf("numToCN(%d) got = %q, want = %q", n, got, want)
		}
	}
}

func TestToHex(t *testing.T) {
	if got, want := toHex("a\n中"), "61a4e2d"; got != want {
		t.Errorf("toHex got = %q, want = %q", got, want)
	}
}

func TestBest(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{[]string{"a.eng.srt", "a.chs.srt", "a.cht.srt"}, "a.chs.srt"},
		{[]string{"a.eng.srt", "dir/a.简英.ass", "dir/.a.中英.ass"}, "dir/a.简英.ass"},
		{[]string{"a.nfo", ".a.srt"}, ""},
		{[]string{"A.SUB"}, "A.SUB"},
	} {
		if got := best(c.names); got != c.want {
			t.Errorf("best(%q) got = %q, want = %q", c.names, got, c.want)
		}
	}
}

func TestBMP(t *testing.T) {
	img, err := decodeBMP(bmp24())
	if err != nil {
		t.Fatal(err)
	}
	if got := [2]color.RGBA{img.RGBAAt(0, 0), img.RGBAAt(1, 0)}; got != [2]color.RGBA{{R: 0xff, A: 0xff}, {B: 0xff, A: 0xff}} {
		t.Errorf("24 bits got = %v", got)
	}
	if img, err = decodeBMP(bmp8()); err != nil {
		t.Fatal(err)
	}
	if got := [2]color.RGBA{img.RGBAAt(0, 0), img.RGBAAt(0, 1)}; got != [2]color.RGBA{{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, {A: 0xff}} {
		t.Errorf("8 bits got = %v", got)
	}
	for _, b := range [][]byte{nil, []byte("BM"), bmp24()[:56]} {
		if _, err := bmpToPNG(b); err == nil {
			t.Errorf("%q got = nil, want an error", b)
		}
	}
}

func TestDefinition(t *testing.T) {
	if _, err := json.Marshal(Definition().Languages); err != nil || Definition().Name != Name {
		t.Errorf("definition got = %+v", Definition())
	}
}
