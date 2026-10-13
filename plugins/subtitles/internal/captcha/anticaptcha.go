package captcha

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// antiCaptcha is Anti-Captcha, as Bazarr's AntiCaptchaProxyLessPitcher
// and AntiCaptchaImageToTextPitcher with python-anticaptcha's client.
type antiCaptcha struct {
	base
	key, url string
}

// antiCaptchaPoll and antiCaptchaJoin are how often and how long
// python-anticaptcha waits for a task's result.
const (
	antiCaptchaPoll = 3 * time.Second
	antiCaptchaJoin = 5 * time.Minute
)

type antiCaptchaError struct {
	id          int
	code, descr string
}

func (e *antiCaptchaError) Error() string {
	return fmt.Sprintf("Anti-Captcha: %s: %s", e.code, e.descr)
}

// errJoinTimeout is a task that took longer than antiCaptchaJoin.
var errJoinTimeout = &antiCaptchaError{code: "timeout", descr: "the task took too long"}

func (a *antiCaptcha) ReCaptchaV2(ctx context.Context, pageURL, siteKey string, invisible bool) (string, error) {
	task := map[string]any{"type": "NoCaptchaTaskProxyless", "websiteURL": pageURL, "websiteKey": siteKey}
	if invisible {
		task["isInvisible"] = true
	}
	return a.solve(ctx, task, "gRecaptchaResponse")
}

func (a *antiCaptcha) Image(ctx context.Context, image []byte) (string, error) {
	return a.solve(ctx, map[string]any{"type": "ImageToTextTask", "body": base64.StdEncoding.EncodeToString(image)}, "text")
}

// solve runs a task, retrying as Bazarr's pitchers: when no worker is
// free and when the task times out, not when the account has no
// balance or the key is wrong.
func (a *antiCaptcha) solve(ctx context.Context, task map[string]any, field string) (string, error) {
	var last error
	for i := range a.tries {
		text, err := a.run(ctx, task, field)
		if err == nil && text != "" {
			return text, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil {
			continue
		}
		last = err
		e, ok := errors.AsType[*antiCaptchaError](err)
		if !ok || i >= a.tries-1 {
			break
		}
		switch e.code {
		case "ERROR_NO_SLOT_AVAILABLE":
			a.log.InfoContext(ctx, "no captcha solving slot available, retrying")
			if err := a.sleep(ctx, 5*time.Second); err != nil {
				return "", err
			}
			continue
		case errJoinTimeout.code:
			a.log.InfoContext(ctx, "captcha solving timed out, retrying")
			if err := a.sleep(ctx, time.Second); err != nil {
				return "", err
			}
			continue
		}
		break
	}
	if last == nil {
		return "", ErrFailed
	}
	return "", fmt.Errorf("%w: %w", ErrFailed, last)
}

// run creates a task and waits for its result.
func (a *antiCaptcha) run(ctx context.Context, task map[string]any, field string) (string, error) {
	var created struct {
		ErrorID          int    `json:"errorId"`
		ErrorCode        string `json:"errorCode"`
		ErrorDescription string `json:"errorDescription"`
		TaskID           int64  `json:"taskId"`
	}
	if err := a.postJSON(ctx, a.url+"/createTask", map[string]any{"clientKey": a.key, "task": task, "languagePool": "en"}, &created); err != nil {
		return "", err
	}
	if created.ErrorID != 0 {
		return "", &antiCaptchaError{id: created.ErrorID, code: created.ErrorCode, descr: created.ErrorDescription}
	}
	for waited := time.Duration(0); ; waited += antiCaptchaPoll {
		var result struct {
			ErrorID          int               `json:"errorId"`
			ErrorCode        string            `json:"errorCode"`
			ErrorDescription string            `json:"errorDescription"`
			Status           string            `json:"status"`
			Solution         map[string]string `json:"solution"`
		}
		if err := a.postJSON(ctx, a.url+"/getTaskResult", map[string]any{"clientKey": a.key, "taskId": created.TaskID}, &result); err != nil {
			return "", err
		}
		if result.ErrorID != 0 {
			return "", &antiCaptchaError{id: result.ErrorID, code: result.ErrorCode, descr: result.ErrorDescription}
		}
		if result.Status == "ready" {
			return result.Solution[field], nil
		}
		if waited >= antiCaptchaJoin {
			return "", errJoinTimeout
		}
		if err := a.sleep(ctx, antiCaptchaPoll); err != nil {
			return "", err
		}
	}
}
