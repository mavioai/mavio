package captcha

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// captchaAI is CaptchaAI's 2Captcha-compatible API, as Bazarr's
// CaptchaAIProxyLessPitcher and CaptchaAIImageToTextPitcher.
type captchaAI struct {
	base
	key, url string
	// timeout is how long a task's result is waited for.
	timeout time.Duration
}

// captchaAIPoll is how often a task's result is asked for.
const captchaAIPoll = 5 * time.Second

func (c *captchaAI) ReCaptchaV2(ctx context.Context, pageURL, siteKey string, _ bool) (string, error) {
	return c.solve(ctx, url.Values{"method": {"userrecaptcha"}, "googlekey": {siteKey}, "pageurl": {pageURL}})
}

func (c *captchaAI) Image(ctx context.Context, image []byte) (string, error) {
	return c.solve(ctx, url.Values{"method": {"base64"}, "body": {base64.StdEncoding.EncodeToString(image)}})
}

type captchaAIAnswer struct {
	Status  int    `json:"status"`
	Request string `json:"request"`
}

// solve submits a captcha and waits for its answer, retrying as Bazarr's
// pitchers: when no worker is free, after other errors of the service and
// after a timeout, not when the account has no balance or the key is
// wrong.
func (c *captchaAI) solve(ctx context.Context, params url.Values) (string, error) {
	params.Set("key", c.key)
	params.Set("json", "1")
	var last error
	for i := range c.tries {
		var in captchaAIAnswer
		err := c.do(ctx, "POST", c.url+"/in.php", "application/x-www-form-urlencoded", strings.NewReader(params.Encode()), &in)
		switch {
		case ctx.Err() != nil:
			return "", ctx.Err()
		case err != nil:
			last = err
			if i >= c.tries-1 {
				return "", fmt.Errorf("%w: CaptchaAI: %w", ErrFailed, last)
			}
			c.log.InfoContext(ctx, "captcha solving failed, retrying", "err", err)
			if err := c.sleep(ctx, 5*time.Second); err != nil {
				return "", err
			}
			continue
		case in.Status != 1:
			last = fmt.Errorf("CaptchaAI: %s", in.Request)
			switch in.Request {
			case "ERROR_ZERO_BALANCE", "ERROR_KEY_DOES_NOT_EXIST", "ERROR_WRONG_USER_KEY":
				return "", fmt.Errorf("%w: %w", ErrFailed, last)
			case "ERROR_NO_SLOT_AVAILABLE":
				if err := c.sleep(ctx, 5*time.Second); err != nil {
					return "", err
				}
			}
			continue
		}
		text, err := c.wait(ctx, in.Request)
		if text != "" || ctx.Err() != nil {
			return text, ctx.Err()
		}
		last = err
	}
	if last == nil {
		return "", ErrFailed
	}
	return "", fmt.Errorf("%w: %w", ErrFailed, last)
}

// wait asks for a task's answer until it is ready or the time is up.
func (c *captchaAI) wait(ctx context.Context, id string) (string, error) {
	q := url.Values{"key": {c.key}, "action": {"get"}, "id": {id}, "json": {"1"}}
	for waited := time.Duration(0); waited < c.timeout; waited += captchaAIPoll {
		if err := c.sleep(ctx, captchaAIPoll); err != nil {
			return "", err
		}
		var res captchaAIAnswer
		if err := c.do(ctx, "GET", c.url+"/res.php?"+q.Encode(), "", nil, &res); err != nil {
			return "", fmt.Errorf("CaptchaAI: %w", err)
		}
		if res.Status == 1 {
			return res.Request, nil
		}
		if res.Request != "CAPCHA_NOT_READY" {
			return "", fmt.Errorf("CaptchaAI: %s", res.Request)
		}
	}
	c.log.InfoContext(ctx, "captcha solving timed out")
	return "", fmt.Errorf("CaptchaAI: timed out")
}
