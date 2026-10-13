// Package captcha solves the captchas of subtitle sites with the captcha
// solving services Bazarr's pitchers use: Anti-Captcha, CaptchaAI and
// DeathByCaptcha, with the administrator's account.
package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// Service names a captcha solving service, as Bazarr's
// anti_captcha_provider setting does.
type Service string

// The services.
const (
	AntiCaptcha    Service = "anti-captcha"
	CaptchaAI      Service = "captchaai"
	DeathByCaptcha Service = "death-by-captcha"
)

// Config is the captcha solving service's account.
type Config struct {
	Service Service `json:"service"`
	// Key is the account key of Anti-Captcha and CaptchaAI.
	Key string `json:"key"`
	// Username and Password are DeathByCaptcha's account.
	Username string `json:"username"`
	Password string `json:"password"`
}

// ErrFailed is a captcha the service did not solve.
var ErrFailed = errors.New("captcha not solved")

// New returns the solver of a configured service; nil when none is
// configured, as Bazarr's configure_captcha_func.
func New(c Config, hc *http.Client, log *slog.Logger) (provider.CaptchaSolver, error) {
	b := base{http: hc, log: log, sleep: sleep, tries: 3}
	switch c.Service {
	case "":
		return nil, nil
	case AntiCaptcha:
		if c.Key == "" {
			return nil, nil
		}
		return &antiCaptcha{base: b, key: c.Key, url: "https://api.anti-captcha.com"}, nil
	case CaptchaAI:
		if c.Key == "" {
			return nil, nil
		}
		return &captchaAI{base: b, key: c.Key, url: "https://ocr.captchaai.com", timeout: 180 * time.Second}, nil
	case DeathByCaptcha:
		if c.Username == "" || c.Password == "" {
			return nil, nil
		}
		return &deathByCaptcha{base: b, username: c.Username, password: c.Password, url: "https://api.dbcapi.me/api"}, nil
	}
	return nil, fmt.Errorf("unknown captcha service %q", c.Service)
}

// base is what the services share.
type base struct {
	http  *http.Client
	log   *slog.Logger
	sleep func(context.Context, time.Duration) error
	// tries is how many times a captcha is tried, as Bazarr's pitchers'.
	tries int
}

// sleep waits for d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// postJSON posts a JSON request and decodes the JSON answer.
func (b *base) postJSON(ctx context.Context, url string, in, out any) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return b.do(ctx, http.MethodPost, url, "application/json", bytes.NewReader(data), out)
}

func (b *base) do(ctx context.Context, method, url, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &statusError{code: resp.StatusCode}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid answer: %w", err)
	}
	return nil
}

type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }
