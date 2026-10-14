package gtranslate

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// microsoftSpec is Microsoft Translator, through the API of its Android
// app, as GTranslate's MicrosoftTranslator.
func microsoftSpec() *spec {
	return &spec{
		name: "microsoft", display: "Microsoft Translator",
		langs:    microsoftLanguages(),
		api:      map[string]string{"pt-BR": "pt"},
		maxChars: 1000,
		new: func(env provider.Env) service {
			return &microsoftService{env: env, scheme: "https://", host: "api.cognitive.microsofttranslator.com", now: time.Now}
		},
	}
}

// microsoftKey is the key the Android app signs its requests with.
const microsoftKey = "\xa2\x29\x3a\x3d\xd0\xdd\x32\x73\x97\x7a\x64\xdb\xc2\xf3\x27\xf5" +
	"\xd7\xbf\x87\xd9\x45\x9d\xf0\x5a\x09\x66\xc6\x30\xc6\x6a\xaa\x84" +
	"\x9a\x41\xaa\x94\x3a\xa8\xd5\x1a\x6e\x4d\xaa\xc9\xa3\x70\x12\x35" +
	"\xc7\xeb\x12\xf6\xe8\x23\x07\x9e\x47\x10\x95\x91\x88\x55\xd8\x17"

type microsoftService struct {
	env          provider.Env
	scheme, host string
	now          func() time.Time
}

func (s *microsoftService) setBase(u string) {
	s.scheme, s.host, _ = strings.Cut(u, "//")
	s.scheme += "//"
}

func (s *microsoftService) text(ctx context.Context, from, to, text string) (string, error) {
	u := s.host + "/translate?" + "api-version=3.0&to=" + url.QueryEscape(to) + "&from=" + url.QueryEscape(from)
	body, err := json.Marshal([]map[string]string{{"Text": text}})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.scheme+u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-MT-Signature", microsoftSignature(u, s.now(), newGUID()))
	resp, err := do(s.env, "Microsoft Translator", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var results []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil || len(results) == 0 || len(results[0].Translations) == 0 {
		return "", provider.Errorf(provider.ParseResponseError, "Microsoft Translator: invalid response")
	}
	return results[0].Translations[0].Text, nil
}

// microsoftSignature signs a request's URL, without its scheme, as the
// Android app does.
func microsoftSignature(u string, now time.Time, guid string) string {
	date := now.UTC().Format("Mon, 02 Jan 2006 15:04:05") + "GMT"
	msg := strings.ToLower("MSTranslatorAndroidApp" + escapeDataString(u) + date + guid)
	mac := hmac.New(sha256.New, []byte(microsoftKey))
	mac.Write([]byte(msg))
	return "MSTranslatorAndroidApp::" + base64.StdEncoding.EncodeToString(mac.Sum(nil)) + "::" + date + "::" + guid
}

// escapeDataString escapes a string as .NET's Uri.EscapeDataString: all
// but RFC 3986's unreserved characters.
func escapeDataString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
