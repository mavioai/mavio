package gtranslate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// googleSpec is Google Translate, through the endpoint of its browser
// extensions, as GTranslate's GoogleTranslator.
func googleSpec() *spec {
	return &spec{
		name: "google", display: "Google Translate",
		langs: translator.Codes(
			"af", "am", "ar", "as", "az", "be", "bg", "bn", "bo", "bs", "ca", "ccp", "ceb", "chr", "ckb", "co", "crk", "crm", "cs", "csw",
			"cy", "da", "de", "dz", "el", "en", "eo", "es", "et", "eu", "fa", "ff", "fi", "fil", "fr", "fy", "ga", "gd", "gl", "gu", "haw",
			"he", "hi", "hmn", "hr", "ht", "hu", "hy", "id", "is", "it", "iu", "ja", "jv", "ka", "kk", "km", "kn", "ko", "ku", "ky",
			"la", "lb", "lis", "lo", "lt", "lv", "mg", "mi", "mk", "ml", "mn", "mni-Mtei", "mr", "ms", "mt", "my", "nb", "ne", "nl", "nn",
			"no", "nv", "ny", "oj", "one", "or", "osa", "pa", "pl", "ps", "pt-BR", "pt-PT", "rhg", "ro", "rom", "ru", "sa", "sd", "see",
			"si", "sk", "sl", "sm", "sn", "so", "sq", "sr", "su", "sv", "sw", "ta", "te", "tg", "th", "ti", "tl", "tr", "tt", "ug", "uk",
			"ur", "uz", "vi", "win", "xh", "yi", "yo", "yue-Hant", "zh-CN", "zh-TW", "zu",
		),
		api:      map[string]string{"pt-BR": "pt", "yue-Hant": "yue"},
		maxChars: 5000,
		new: func(env provider.Env) service {
			return &googleService{env: env, base: "https://translate.googleapis.com", now: time.Now}
		},
	}
}

type googleService struct {
	env  provider.Env
	base string
	now  func() time.Time
}

func (s *googleService) setBase(u string) { s.base = u }

// googleResult is Google's answer.
type googleResult struct {
	Sentences []struct {
		Trans string `json:"trans"`
	} `json:"sentences"`
}

func (s *googleService) text(ctx context.Context, from, to, text string) (string, error) {
	q := url.Values{"client": {"gtx"}, "sl": {from}, "tl": {to}, "dt": {"t", "bd"}, "dj": {"1"}, "source": {"input"}, "tk": {googleToken(text, s.now())}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/translate_a/single?"+q.Encode(), strings.NewReader(url.Values{"q": {text}}.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	resp, err := do(s.env, "Google Translate", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r googleResult
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", provider.Errorf(provider.ParseResponseError, "Google Translate: %w", err)
	}
	var b strings.Builder
	for _, sentence := range r.Sentences {
		b.WriteString(sentence.Trans)
	}
	return b.String(), nil
}

// googleToken is the token Google's requests carry for a text, as
// GTranslate's MakeToken.
func googleToken(text string, now time.Time) string {
	a := now.Unix() / 3600
	b := a
	for _, ch := range utf16.Encode([]rune(text)) {
		a = googleWork(a+int64(ch), "+-a^+6")
	}
	a = googleWork(a, "+-3^+b+-f")
	if a < 0 {
		a = (a & 0x7fffffff) + 0x7fffffff + 1
	}
	a %= 1000000
	return fmt.Sprintf("%d.%d", a, a^b)
}

func googleWork(num int64, seed string) int64 {
	for i := 0; i < len(seed)-2; i += 3 {
		d := int64(seed[i+2])
		if d >= 'a' {
			d -= 'W'
		}
		if seed[i+1] == '+' {
			num = (num + (num >> d)) & 0xffffffff
		} else {
			num ^= num << d
		}
	}
	return num
}
