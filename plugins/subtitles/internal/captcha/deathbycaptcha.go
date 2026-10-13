package captcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

// deathByCaptcha is DeathByCaptcha's HTTP API, as Bazarr's
// DBCProxyLessPitcher and DBCImageToTextPitcher with its client.
type deathByCaptcha struct {
	base
	username, password, url string
}

// DeathByCaptcha's client waits this long for tokens and images, polling
// at these intervals and then every 3 seconds.
const (
	dbcTokenTimeout = 120 * time.Second
	dbcImageTimeout = 60 * time.Second
)

var dbcPolls = []time.Duration{1, 1, 2, 3, 2, 2, 3, 2, 2}

func dbcPoll(i int) time.Duration {
	if i < len(dbcPolls) {
		return dbcPolls[i] * time.Second
	}
	return 3 * time.Second
}

type dbcCaptcha struct {
	Captcha   int64  `json:"captcha"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct"`
}

func (d *deathByCaptcha) ReCaptchaV2(ctx context.Context, pageURL, siteKey string, _ bool) (string, error) {
	params, err := json.Marshal(map[string]string{"googlekey": siteKey, "pageurl": pageURL})
	if err != nil {
		return "", err
	}
	return d.solve(ctx, map[string]string{"type": "4", "token_params": string(params)}, nil, dbcTokenTimeout)
}

func (d *deathByCaptcha) Image(ctx context.Context, image []byte) (string, error) {
	return d.solve(ctx, nil, image, dbcImageTimeout)
}

// solve uploads a captcha and polls for its text, trying again as
// Bazarr's pitchers.
func (d *deathByCaptcha) solve(ctx context.Context, fields map[string]string, image []byte, timeout time.Duration) (string, error) {
	var last error
	for range d.tries {
		c, err := d.decode(ctx, fields, image, timeout)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil && c.Text != "" && c.IsCorrect {
			return c.Text, nil
		}
		last = err
		var se *statusError
		if errors.As(err, &se) && se.code == http.StatusForbidden {
			// Access denied: the credentials or the balance.
			break
		}
	}
	if last == nil {
		return "", ErrFailed
	}
	return "", fmt.Errorf("%w: DeathByCaptcha: %w", ErrFailed, last)
}

// decode is the client's decode: an upload, then polls until the text
// comes or the time is up.
func (d *deathByCaptcha) decode(ctx context.Context, fields map[string]string, image []byte, timeout time.Duration) (dbcCaptcha, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"username": d.username, "password": d.password} {
		if err := w.WriteField(k, v); err != nil {
			return dbcCaptcha{}, err
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return dbcCaptcha{}, err
		}
	}
	if image != nil {
		if err := w.WriteField("captchafile", "base64:"+base64.StdEncoding.EncodeToString(image)); err != nil {
			return dbcCaptcha{}, err
		}
	}
	if err := w.Close(); err != nil {
		return dbcCaptcha{}, err
	}
	var c dbcCaptcha
	if err := d.do(ctx, http.MethodPost, d.url+"/captcha", w.FormDataContentType(), &body, &c); err != nil {
		return dbcCaptcha{}, err
	}
	for i, waited := 0, time.Duration(0); c.Text == "" && waited < timeout; i++ {
		p := dbcPoll(i)
		if err := d.sleep(ctx, p); err != nil {
			return dbcCaptcha{}, err
		}
		waited += p
		var pulled dbcCaptcha
		if err := d.do(ctx, http.MethodGet, d.url+"/captcha/"+strconv.FormatInt(c.Captcha, 10), "", nil, &pulled); err != nil {
			return dbcCaptcha{}, err
		}
		if pulled.Captcha == c.Captcha {
			c = pulled
		}
	}
	return c, nil
}
