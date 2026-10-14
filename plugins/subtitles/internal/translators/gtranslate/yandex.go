package gtranslate

import (
	"context"
	"encoding/json"
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

// yandexSpec is Yandex.Translate, through the API of its Android app, as
// GTranslate's YandexTranslator.
func yandexSpec() *spec {
	return &spec{
		name: "yandex", display: "Yandex.Translate",
		langs: append(translator.Codes(
			"af", "sq", "am", "ar", "hy", "az", "ba", "eu", "be", "bn", "bs", "bg", "my", "ca", "ceb", "zh", "cv", "hr", "cs", "da",
			"nl", "sjn", "en", "eo", "et", "fi", "fr", "gl", "ka", "de", "el", "gu", "ht", "he", "mrj", "hi", "hu", "is", "id", "ga",
			"it", "ja", "jv", "kn", "kk", "km", "ko", "ky", "lo", "la", "lv", "lt", "lb", "mk", "mg", "ms", "ml", "mt", "mi", "mr",
			"mhr", "mn", "ne", "no", "pap", "fa", "pl", "pt-PT", "pt-BR", "pa", "os", "ro", "ru", "gd", "sr", "sr-Latn", "si", "sk",
			"sl", "es", "su", "sw", "sv", "tl", "tg", "ta", "tt", "te", "th", "tr", "udm", "uk", "ur", "uz", "vi", "cy", "xh", "sah",
			"yi", "zu",
		), translator.Language{Code: "kazlat", Tag: "kk-Latn"}, translator.Language{Code: "uzbcyr", Tag: "uz-Cyrl"}),
		api: map[string]string{"pt-PT": "pt"},
		new: func(env provider.Env) service {
			return &yandexService{env: env, base: "https://translate.yandex.net", now: time.Now}
		},
	}
}

// yandexUserAgent is the Android app's.
const yandexUserAgent = "ru.yandex.translate/3.20.2024"

type yandexService struct {
	env  provider.Env
	base string
	now  func() time.Time

	mu         sync.Mutex
	ucid       string
	ucidExpiry time.Time
}

func (s *yandexService) setBase(u string) { s.base = u }

// clientID returns the ID of the app's install, a new one every six
// minutes.
func (s *yandexService) clientID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now := s.now(); s.ucid == "" || !now.Before(s.ucidExpiry) {
		s.ucid, s.ucidExpiry = newGUID(), now.Add(6*time.Minute)
	}
	return s.ucid
}

func (s *yandexService) text(ctx context.Context, from, to, text string) (string, error) {
	q := url.Values{"ucid": {s.clientID()}, "srv": {"android"}, "format": {"text"}}
	form := url.Values{"text": {text}, "lang": {from + "-" + to}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+"/api/v1/tr.json/translate?"+q.Encode(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", yandexUserAgent)
	resp, err := s.env.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	defer resp.Body.Close()
	// Yandex answers its failures' status in the body too.
	var r struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Text    []string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", translator.StatusError("Yandex.Translate", resp)
		}
		return "", provider.Errorf(provider.ParseResponseError, "Yandex.Translate: %w", err)
	}
	if r.Code != http.StatusOK {
		return "", translator.StatusError("Yandex.Translate", &http.Response{
			StatusCode: r.Code, Status: strconv.Itoa(r.Code), Body: io.NopCloser(strings.NewReader(r.Message)),
		})
	}
	if len(r.Text) == 0 {
		return "", provider.Errorf(provider.ParseResponseError, "Yandex.Translate: no translation")
	}
	return r.Text[0], nil
}
