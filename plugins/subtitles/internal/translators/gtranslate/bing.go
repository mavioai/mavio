package gtranslate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

// microsoftLanguages are the languages of Bing and Microsoft Translator,
// as their APIs name them.
func microsoftLanguages() []translator.Language {
	return translator.Codes(
		"af", "am", "ar", "as", "az", "ba", "bg", "bho", "bn", "bo", "brx", "bs", "ca", "cs", "cy", "da", "de", "doi", "dsb", "dv",
		"el", "en", "es", "et", "eu", "fa", "fi", "fil", "fj", "fo", "fr", "fr-CA", "ga", "gl", "gom", "gu", "ha", "he", "hi", "hne",
		"hr", "hsb", "ht", "hu", "hy", "id", "ig", "ikt", "is", "it", "iu", "iu-Latn", "ja", "ka", "kk", "km", "kmr", "kn", "ko",
		"ks", "ku", "ky", "ln", "lo", "lt", "lug", "lv", "lzh", "mai", "mg", "mi", "mk", "ml", "mn-Cyrl", "mn-Mong", "mni", "mr",
		"ms", "mt", "mww", "my", "nb", "ne", "nl", "nso", "nya", "or", "otq", "pa", "pl", "prs", "ps", "pt-BR", "pt-PT", "ro", "ru",
		"run", "rw", "sd", "si", "sk", "sl", "sm", "sn", "so", "sq", "sr-Cyrl", "sr-Latn", "st", "sv", "sw", "ta", "te", "th", "ti",
		"tk", "tlh-Latn", "tlh-Piqd", "tn", "to", "tr", "tt", "ty", "ug", "uk", "ur", "uz", "vi", "xh", "yo", "yua", "yue",
		"zh-Hans", "zh-Hant", "zu",
	)
}

// bingSpec is Bing Translator, through the endpoint of its page with the
// page's credentials, as GTranslate's BingTranslator.
func bingSpec() *spec {
	return &spec{
		name: "bing", display: "Bing Translator",
		langs:    microsoftLanguages(),
		api:      map[string]string{"pt-BR": "pt"},
		maxChars: 1000,
		new: func(env provider.Env) service {
			return &bingService{env: env, base: "https://www.bing.com", now: time.Now}
		},
	}
}

// bingIID identifies the translator page's requests.
const bingIID = "translator.5024.1"

// bingCredentials are the credentials the translator page holds.
type bingCredentials struct {
	token string
	key   int64
	ig    string
	until time.Time
}

type bingService struct {
	env  provider.Env
	base string
	now  func() time.Time

	mu    sync.Mutex
	creds *bingCredentials
}

func (s *bingService) setBase(u string) { s.base = u }

// credentials returns the page's credentials, read again once they
// expire an hour after the page gave them.
func (s *bingService) credentials(ctx context.Context) (*bingCredentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.creds != nil && s.now().Before(s.creds.until) {
		return s.creds, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/translator", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := do(s.env, "Bing Translator", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bing translator: %w", err)
	}
	c, err := bingParseCredentials(page, s.now())
	if err != nil {
		return nil, err
	}
	s.creds = c
	return c, nil
}

// bingParseCredentials reads the key and token of the translator page's
// params_AbusePreventionHelper.
func bingParseCredentials(page []byte, now time.Time) (*bingCredentials, error) {
	const start = "var params_AbusePreventionHelper = ["
	i := bytes.Index(page, []byte(start))
	if i < 0 {
		return nil, provider.Errorf(provider.ParseResponseError, "Bing Translator: no credentials in the page")
	}
	rest := page[i+len(start):]
	comma := bytes.IndexByte(rest, ',')
	if comma < 0 || len(rest) < comma+2 {
		return nil, provider.Errorf(provider.ParseResponseError, "Bing Translator: no key in the page")
	}
	key, err := strconv.ParseInt(string(rest[:comma]), 10, 64)
	if err != nil {
		key = now.UnixMilli()
	}
	rest = rest[comma+2:]
	quote := bytes.IndexByte(rest, '"')
	if quote < 0 {
		return nil, provider.Errorf(provider.ParseResponseError, "Bing Translator: no token in the page")
	}
	return &bingCredentials{
		token: string(rest[:quote]), key: key, ig: strings.ToUpper(newGUID()),
		until: time.UnixMilli(key + time.Hour.Milliseconds()),
	}, nil
}

func (s *bingService) text(ctx context.Context, from, to, text string) (string, error) {
	c, err := s.credentials(ctx)
	if err != nil {
		return "", err
	}
	form := url.Values{"fromLang": {from}, "text": {text}, "to": {to}, "token": {c.token}, "key": {strconv.FormatInt(c.key, 10)}}
	q := url.Values{"isVertical": {"1"}, "IG": {c.ig}, "IID": {bingIID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/ttranslatev3?"+q.Encode(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	resp, err := do(s.env, "Bing Translator", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("bing translator: %w", err)
	}
	// Bing answers 200 with the status of its failures.
	var failure struct {
		StatusCode int    `json:"statusCode"`
		Message    string `json:"message"`
	}
	if json.Unmarshal(data, &failure) == nil && failure.StatusCode != 0 {
		if failure.StatusCode == http.StatusUnauthorized || failure.StatusCode == http.StatusForbidden {
			// The credentials expired early: read again when retried.
			s.mu.Lock()
			s.creds = nil
			s.mu.Unlock()
			return "", provider.Errorf(provider.ServiceUnavailable, "Bing Translator: credentials rejected")
		}
		return "", translator.StatusError("Bing Translator", &http.Response{
			StatusCode: failure.StatusCode, Status: strconv.Itoa(failure.StatusCode), Body: io.NopCloser(strings.NewReader(failure.Message)),
		})
	}
	var results []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(data, &results); err != nil || len(results) == 0 || len(results[0].Translations) == 0 {
		return "", provider.Errorf(provider.ParseResponseError, "Bing Translator: invalid response")
	}
	return results[0].Translations[0].Text, nil
}
