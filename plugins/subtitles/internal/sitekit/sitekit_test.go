package sitekit

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestDecode(t *testing.T) {
	for _, c := range []struct {
		name string
		in   []byte
		want string
	}{
		{"utf-8 kept", []byte("Καλημέρα"), "Καλημέρα"},
		{"windows-1253", []byte{0xca, 0xe1, 0xeb, 0xe7, 0xec, 0xdd, 0xf1, 0xe1}, "Καλημέρα"},
		{"ascii", []byte("hello"), "hello"},
	} {
		if got := string(Decode(c.in, charmap.Windows1253)); got != c.want {
			t.Errorf("%s: Decode got = %q, want = %q", c.name, got, c.want)
		}
	}
}

func TestFramerateEqual(t *testing.T) {
	for _, c := range []struct {
		a, b float64
		want bool
	}{
		{25, 25, true},
		{23.976, 24, true},
		{23.98, 23.976, true},
		{23.976, 25, false},
		{29.97, 30, false},
	} {
		if got := FramerateEqual(c.a, c.b); got != c.want {
			t.Errorf("FramerateEqual(%v, %v) got = %v, want = %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseCookies(t *testing.T) {
	got := ParseCookies(` a=1; b="two words" ;=x; c; d= 4 `)
	var pairs []string
	for _, c := range got {
		if c.Path != "/" {
			t.Errorf("cookie %s: Path got = %q, want = %q", c.Name, c.Path, "/")
		}
		pairs = append(pairs, c.Name+"="+c.Value)
	}
	if want := []string{"a=1", "b=two words", "d=4"}; !slices.Equal(pairs, want) {
		t.Errorf("ParseCookies got = %q, want = %q", pairs, want)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "provider")
	type state struct{ N int }

	var s state
	if ok, err := Load(dir, "state", &s); ok || err != nil {
		t.Fatalf("Load before Save got = %v, %v, want = false, nil", ok, err)
	}
	if err := Save(dir, "state", state{N: 3}); err != nil {
		t.Fatal(err)
	}
	if ok, err := Load(dir, "state", &s); !ok || err != nil || s.N != 3 {
		t.Fatalf("Load got = %v, %v, %+v, want = true, nil, {N:3}", ok, err, s)
	}
	if names, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(names) != 0 {
		t.Errorf("temporary files left: %q", names)
	}
	if err := Remove(dir, "state"); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, "state"); err != nil {
		t.Errorf("Remove of a missing state got = %v, want = nil", err)
	}
	if ok, _ := Load(dir, "state", &s); ok {
		t.Error("Load after Remove got = true, want = false")
	}

	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, "bad", &s); err == nil {
		t.Error("Load of bad JSON got = nil error, want one")
	}

	// Without a data folder nothing is kept.
	if err := Save("", "state", state{N: 1}); err != nil {
		t.Errorf("Save without a folder got = %v, want = nil", err)
	}
	if ok, err := Load("", "state", &s); ok || err != nil {
		t.Errorf("Load without a folder got = %v, %v, want = false, nil", ok, err)
	}
}

func TestLogin(t *testing.T) {
	dir := t.TempDir()
	u, _ := url.Parse("https://site.example/")
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	jar, _ := cookiejar.New(nil)
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "abc"}})
	if err := SaveLogin(dir, "login", jar, u, "agent", now); err != nil {
		t.Fatal(err)
	}

	fresh, _ := cookiejar.New(nil)
	l, ok, err := LoadLogin(dir, "login", fresh, u, time.Hour, now.Add(30*time.Minute))
	if err != nil || !ok {
		t.Fatalf("LoadLogin got = %v, %v, want = true, nil", ok, err)
	}
	if l.UserAgent != "agent" {
		t.Errorf("UserAgent got = %q, want = %q", l.UserAgent, "agent")
	}
	if cs := fresh.Cookies(u); len(cs) != 1 || cs[0].Name != "session" || cs[0].Value != "abc" {
		t.Errorf("cookies got = %v, want = [session=abc]", cs)
	}

	stale, _ := cookiejar.New(nil)
	if _, ok, err := LoadLogin(dir, "login", stale, u, time.Hour, now.Add(2*time.Hour)); ok || err != nil {
		t.Errorf("LoadLogin of an old login got = %v, %v, want = false, nil", ok, err)
	}
	if cs := stale.Cookies(u); len(cs) != 0 {
		t.Errorf("cookies of an old login got = %v, want none", cs)
	}
}

func TestRecaptcha(t *testing.T) {
	for _, c := range []struct {
		page, field, key string
		has              bool
	}{
		{`<div class="g-recaptcha" data-sitekey="KEY1"></div>`, "g-recaptcha-response", "KEY1", true},
		{`<script>grecaptcha.execute('KEY2', {action: 'x'})</script>`, "recaptcha_response", "KEY2", true},
		{`<form></form>`, "", "", false},
	} {
		if got := HasRecaptcha(c.page); got != c.has {
			t.Errorf("HasRecaptcha(%q) got = %v, want = %v", c.page, got, c.has)
		}
		if field, key := RecaptchaKey(c.page); field != c.field || key != c.key {
			t.Errorf("RecaptchaKey(%q) got = %q, %q, want = %q, %q", c.page, field, key, c.field, c.key)
		}
	}
}

func TestMatch(t *testing.T) {
	langs := []provider.Language{{Tag: "en", Forced: true}, {Tag: "pt-BR", HearingImpaired: true}}
	if l, ok := Match(langs, "pt-br"); !ok || l != (provider.Language{Tag: "pt-BR"}) {
		t.Errorf("Match(pt-br) got = %+v, %v, want = {pt-BR}, true", l, ok)
	}
	if _, ok := Match(langs, "en"); ok {
		t.Error("Match(en) of a forced request got = true, want = false")
	}
}

func TestLanguageByName(t *testing.T) {
	for name, want := range map[string]string{
		"Brazilian Portuguese": "pt-BR",
		" Hebrew ":             "he",
		"English":              "en",
		"Klingonese":           "",
	} {
		if got := LanguageByName(name); got != want {
			t.Errorf("LanguageByName(%q) got = %q, want = %q", name, got, want)
		}
	}
}

func TestCaptcha(t *testing.T) {
	_, err := Captcha(provider.Env{}, "Site")
	if provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("Captcha without a solver got = %v, want a configuration error", err)
	}
}
