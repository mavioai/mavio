package openai

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

// Name is the recognizer's and the synthesizer's name.
const Name = "openai"

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 600
)

// connection is how a service serving the API is reached.
type connection struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Timeout *int   `json:"timeout"`
}

func connectionSettings(model, help string) []service.Setting {
	return []service.Setting{
		{Name: "base_url", Label: "Base URL", Help: "The API's base URL, such as Groq's or a speaches server's.", Type: service.Text, Default: defaultBaseURL},
		{Name: "api_key", Label: "API key", Type: service.Password},
		{Name: "model", Label: "Model", Help: help, Type: service.Text, Default: model},
		{Name: "timeout", Label: "Timeout in seconds", Type: service.Number, Default: defaultTimeout, Advanced: true},
	}
}

// client is a client of the API.
type client struct {
	base, key, model string
	http             *http.Client
}

func newClient(env service.Env, c connection, model string) (*client, error) {
	if c.BaseURL = strings.TrimSuffix(strings.TrimSpace(c.BaseURL), "/"); c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	if u, err := url.Parse(c.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: base URL %q", service.ErrConfig, c.BaseURL)
	}
	if c.Model = strings.TrimSpace(c.Model); c.Model == "" {
		c.Model = model
	}
	timeout := defaultTimeout
	if c.Timeout != nil {
		timeout = *c.Timeout
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("%w: the timeout must be positive", service.ErrConfig)
	}
	hc := http.DefaultClient
	if env.HTTP != nil {
		hc = env.HTTP
	}
	h := *hc
	h.Timeout = time.Duration(timeout) * time.Second
	return &client{base: c.BaseURL, key: c.APIKey, model: c.Model, http: &h}, nil
}

func (c *client) do(ctx context.Context, path, contentType string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", service.ErrService, err)
	}
	return service.Answer(resp)
}

// form is a multipart form being written.
type form struct {
	buf bytes.Buffer
	w   *multipart.Writer
}

func newForm() *form {
	f := &form{}
	f.w = multipart.NewWriter(&f.buf)
	return f
}

func (f *form) field(name, value string) {
	if value != "" {
		_ = f.w.WriteField(name, value)
	}
}

func (f *form) file(name, file string, data []byte) {
	part, _ := f.w.CreateFormFile(name, file)
	_, _ = part.Write(data)
}

func (f *form) close() ([]byte, string) {
	_ = f.w.Close()
	return f.buf.Bytes(), f.w.FormDataContentType()
}
