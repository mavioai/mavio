package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fake is a captcha service answering its endpoints from scripts, one
// answer per request, the last repeated.
type fake struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	answers  map[string][]string
	requests map[string][]string
}

func newFake(t *testing.T, answers map[string][]string) *fake {
	f := &fake{t: t, answers: answers, requests: map[string][]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body string
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			body = fmt.Sprint(r.MultipartForm.Value)
		} else {
			data := make([]byte, 1<<16)
			n, _ := r.Body.Read(data)
			body = r.URL.RawQuery + string(data[:n])
		}
		path := r.URL.Path
		if strings.HasPrefix(path, "/captcha/") {
			path = "/captcha/id"
		}
		f.mu.Lock()
		f.requests[path] = append(f.requests[path], body)
		list := f.answers[path]
		var answer string
		if len(list) > 0 {
			answer = list[0]
			if len(list) > 1 {
				f.answers[path] = list[1:]
			}
		}
		f.mu.Unlock()
		if code, ok := strings.CutPrefix(answer, "HTTP "); ok {
			var n int
			_, _ = fmt.Sscan(code, &n)
			w.WriteHeader(n)
			return
		}
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests[path])
}

func (f *fake) base() base {
	return base{http: f.srv.Client(), log: slog.New(slog.DiscardHandler), sleep: func(context.Context, time.Duration) error { return nil }, tries: 3}
}

func TestAntiCaptcha(t *testing.T) {
	f := newFake(t, map[string][]string{
		"/createTask": {
			`{"errorId":2,"errorCode":"ERROR_NO_SLOT_AVAILABLE","errorDescription":"busy"}`,
			`{"errorId":0,"taskId":7}`,
		},
		"/getTaskResult": {`{"status":"processing"}`, `{"status":"ready","solution":{"gRecaptchaResponse":"token"}}`},
	})
	a := &antiCaptcha{base: f.base(), key: "k", url: f.srv.URL}
	got, err := a.ReCaptchaV2(t.Context(), "https://site/login.php", "site-key", true)
	if err != nil || got != "token" {
		t.Fatalf("ReCaptchaV2 = %q, %v, want = token", got, err)
	}
	var task struct {
		ClientKey string         `json:"clientKey"`
		Task      map[string]any `json:"task"`
	}
	if err := json.Unmarshal([]byte(f.requests["/createTask"][1]), &task); err != nil {
		t.Fatal(err)
	}
	if task.ClientKey != "k" || task.Task["type"] != "NoCaptchaTaskProxyless" || task.Task["websiteKey"] != "site-key" || task.Task["isInvisible"] != true {
		t.Errorf("task = %+v", task)
	}
	if n := f.count("/getTaskResult"); n != 2 {
		t.Errorf("polls = %d, want = 2", n)
	}

	f = newFake(t, map[string][]string{"/createTask": {`{"errorId":10,"errorCode":"ERROR_ZERO_BALANCE","errorDescription":"no money"}`}})
	a = &antiCaptcha{base: f.base(), key: "k", url: f.srv.URL}
	if _, err := a.Image(t.Context(), []byte("png")); !errors.Is(err, ErrFailed) || !strings.Contains(err.Error(), "ERROR_ZERO_BALANCE") {
		t.Errorf("Image without balance: err = %v, want = %v", err, ErrFailed)
	}
	if n := f.count("/createTask"); n != 1 {
		t.Errorf("tasks without balance = %d, want = 1", n)
	}

	f = newFake(t, map[string][]string{
		"/createTask":    {`{"errorId":0,"taskId":8}`},
		"/getTaskResult": {`{"status":"ready","solution":{"text":"abcd"}}`},
	})
	a = &antiCaptcha{base: f.base(), key: "k", url: f.srv.URL}
	if got, err := a.Image(t.Context(), []byte("png")); err != nil || got != "abcd" {
		t.Errorf("Image = %q, %v, want = abcd", got, err)
	}
	if !strings.Contains(f.requests["/createTask"][0], `"body":"cG5n"`) {
		t.Errorf("image task = %s", f.requests["/createTask"][0])
	}
}

func TestCaptchaAI(t *testing.T) {
	f := newFake(t, map[string][]string{
		"/in.php":  {`{"status":0,"request":"ERROR_NO_SLOT_AVAILABLE"}`, `{"status":1,"request":"42"}`},
		"/res.php": {`{"status":0,"request":"CAPCHA_NOT_READY"}`, `{"status":1,"request":"token"}`},
	})
	c := &captchaAI{base: f.base(), key: "k", url: f.srv.URL, timeout: time.Minute}
	got, err := c.ReCaptchaV2(t.Context(), "https://site/", "site-key", false)
	if err != nil || got != "token" {
		t.Fatalf("ReCaptchaV2 = %q, %v, want = token", got, err)
	}
	in := f.requests["/in.php"][1]
	for _, want := range []string{"method=userrecaptcha", "googlekey=site-key", "key=k", "json=1"} {
		if !strings.Contains(in, want) {
			t.Errorf("in.php = %s, want %s", in, want)
		}
	}
	if res := f.requests["/res.php"][0]; !strings.Contains(res, "action=get") || !strings.Contains(res, "id=42") {
		t.Errorf("res.php = %s", res)
	}

	f = newFake(t, map[string][]string{"/in.php": {`{"status":0,"request":"ERROR_WRONG_USER_KEY"}`}})
	c = &captchaAI{base: f.base(), key: "k", url: f.srv.URL, timeout: time.Minute}
	if _, err := c.Image(t.Context(), []byte("png")); !errors.Is(err, ErrFailed) {
		t.Errorf("Image with a wrong key: err = %v, want = %v", err, ErrFailed)
	}
	if n := f.count("/in.php"); n != 1 {
		t.Errorf("submissions with a wrong key = %d, want = 1", n)
	}

	// A task never ready times out, and is tried three times.
	f = newFake(t, map[string][]string{"/in.php": {`{"status":1,"request":"1"}`}, "/res.php": {`{"status":0,"request":"CAPCHA_NOT_READY"}`}})
	c = &captchaAI{base: f.base(), key: "k", url: f.srv.URL, timeout: 10 * time.Second}
	if _, err := c.Image(t.Context(), []byte("png")); !errors.Is(err, ErrFailed) {
		t.Errorf("Image never ready: err = %v, want = %v", err, ErrFailed)
	}
	if n, m := f.count("/in.php"), f.count("/res.php"); n != 3 || m != 6 {
		t.Errorf("submissions, polls = %d, %d, want = 3, 6", n, m)
	}
}

func TestDeathByCaptcha(t *testing.T) {
	f := newFake(t, map[string][]string{
		"/captcha":    {`{"captcha":5,"text":"","is_correct":true}`},
		"/captcha/id": {`{"captcha":5,"text":"","is_correct":true}`, `{"captcha":5,"text":"token","is_correct":true}`},
	})
	d := &deathByCaptcha{base: f.base(), username: "u", password: "p", url: f.srv.URL}
	got, err := d.ReCaptchaV2(t.Context(), "https://site/", "site-key", false)
	if err != nil || got != "token" {
		t.Fatalf("ReCaptchaV2 = %q, %v, want = token", got, err)
	}
	up := f.requests["/captcha"][0]
	for _, want := range []string{"username:[u]", "password:[p]", "type:[4]", `"googlekey":"site-key"`} {
		if !strings.Contains(up, want) {
			t.Errorf("upload = %s, want %s", up, want)
		}
	}

	f = newFake(t, map[string][]string{"/captcha": {"HTTP 403"}})
	d = &deathByCaptcha{base: f.base(), username: "u", password: "p", url: f.srv.URL}
	if _, err := d.Image(t.Context(), []byte("png")); !errors.Is(err, ErrFailed) {
		t.Errorf("Image denied: err = %v, want = %v", err, ErrFailed)
	}
	if n := f.count("/captcha"); n != 1 {
		t.Errorf("uploads denied = %d, want = 1", n)
	}

	// A wrong answer is tried again.
	f = newFake(t, map[string][]string{"/captcha": {`{"captcha":6,"text":"x","is_correct":false}`, `{"captcha":7,"text":"abcd","is_correct":true}`}})
	d = &deathByCaptcha{base: f.base(), username: "u", password: "p", url: f.srv.URL}
	if got, err := d.Image(t.Context(), []byte("png")); err != nil || got != "abcd" {
		t.Errorf("Image = %q, %v, want = abcd", got, err)
	}
	if up := f.requests["/captcha"][1]; !strings.Contains(up, "captchafile:[base64:cG5n]") {
		t.Errorf("upload = %s", up)
	}
}

func TestNew(t *testing.T) {
	for _, c := range []struct {
		cfg  Config
		want string
	}{
		{Config{}, "<nil>"},
		{Config{Service: AntiCaptcha}, "<nil>"},
		{Config{Service: AntiCaptcha, Key: "k"}, "*captcha.antiCaptcha"},
		{Config{Service: CaptchaAI, Key: "k"}, "*captcha.captchaAI"},
		{Config{Service: DeathByCaptcha, Username: "u"}, "<nil>"},
		{Config{Service: DeathByCaptcha, Username: "u", Password: "p"}, "*captcha.deathByCaptcha"},
	} {
		s, err := New(c.cfg, http.DefaultClient, slog.New(slog.DiscardHandler))
		if got := fmt.Sprintf("%T", s); err != nil || got != c.want {
			t.Errorf("New(%+v) = %s, %v, want = %s", c.cfg, got, err, c.want)
		}
	}
	if _, err := New(Config{Service: "2captcha"}, http.DefaultClient, nil); err == nil {
		t.Error("New of an unknown service: no error")
	}
}
