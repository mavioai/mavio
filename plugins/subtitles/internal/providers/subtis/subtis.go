// Package subtis is the provider of Subtis, ported from Bazarr's subtis:
// Spanish subtitles of movies found by the video file's hash, size, name,
// and then by an alternative, fuzzy match of the name.
package subtis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's name.
const Name = "subtis"

// version is Bazarr's provider's version, sent in the User-Agent.
const version = "0.9.2"

// language is the provider's only language.
const language = "es"

// Definition returns the provider's definition.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Subtis",
		Languages:      []string{language},
		HashVerifiable: true, Movies: true,
		New: New,
	}
}

// New makes the provider; it takes no settings.
func New(env provider.Env, _ json.RawMessage) (provider.Provider, error) {
	s := site.New(env.HTTP)
	s.Header.Set("User-Agent", "Bazarr/Subtis/"+version)
	s.Header.Set("Accept", "application/json")
	return &Provider{env: env, s: s, base: "https://api.subt.is/v1"}, nil
}

// Provider is Subtis.
type Provider struct {
	env  provider.Env
	s    *site.Session
	base string
}

// SetBaseURL points the provider at another server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) { p.base = strings.TrimSuffix(u.String(), "/") + "/v1" }

// escape quotes a file name as Python's quote with no safe characters.
func escape(name string) string { return strings.ReplaceAll(url.QueryEscape(name), "+", "%20") }

// step is a search of Bazarr's cascade.
type step struct {
	url    string
	synced bool
	method string
}

// Search implements provider.Provider, as Bazarr's query: the first
// subtitle the cascade of searches by hash, size, name and alternative
// name finds, for each language.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.Episode {
		return nil, nil
	}
	if v.FileName == "" {
		p.env.Log.WarnContext(ctx, "missing video name")
		return nil, nil
	}
	name := filepath.Base(strings.ReplaceAll(v.FileName, `\`, "/"))
	var steps []step
	if v.FileHash != "" {
		steps = append(steps, step{p.base + "/subtitle/find/file/hash/" + v.FileHash, true, "hash"})
	}
	if v.FileSize > 0 {
		steps = append(steps, step{p.base + "/subtitle/find/file/bytes/" + strconv.FormatInt(v.FileSize, 10), true, "bytes"})
	}
	steps = append(steps,
		step{p.base + "/subtitle/find/file/name/" + escape(name), true, "name"},
		step{p.base + "/subtitle/file/alternative/" + escape(name), false, "alternative"})

	var out []provider.Subtitle
	for _, l := range langs {
		if l.Tag != language {
			continue
		}
		for _, st := range steps {
			link, title, err := p.fetch(ctx, st.url, name)
			if err != nil {
				return nil, err
			}
			if link == "" {
				continue
			}
			p.env.Log.DebugContext(ctx, "found subtitle via cascade search", "method", st.method, "file", name)
			sub := provider.Subtitle{
				ID:           link,
				Language:     provider.Language{Tag: language, Forced: l.Forced},
				ReleaseNames: []string{title},
				PageLink:     st.url,
				Format:       "srt",
			}
			if st.method == "hash" {
				sub.Matches = []string{"hash"}
			}
			if !st.synced {
				sub.Comment = "fuzzy match"
			}
			out = append(out, sub)
			break
		}
	}
	return out, nil
}

// fetch asks the API for a subtitle, as Bazarr's _fetch_subtitle: its
// link and title, "" when there is none; failures are logged and taken for
// none, but a canceled search.
func (p *Provider) fetch(ctx context.Context, u, name string) (link, title string, err error) {
	r, err := p.s.Get(ctx, u, nil)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", "", ctxErr
		}
		if r == nil || r.Status != http.StatusNotFound {
			p.env.Log.WarnContext(ctx, "subtis search failed", "file", name, "err", err)
		}
		return "", "", nil
	}
	var res struct {
		Subtitle *struct {
			Link any `json:"subtitle_link"`
		} `json:"subtitle"`
		Title json.RawMessage `json:"title"`
	}
	if err := json.Unmarshal(r.Body, &res); err != nil {
		p.env.Log.WarnContext(ctx, "invalid JSON response", "file", name, "err", err)
		return "", "", nil
	}
	if res.Subtitle == nil {
		return "", "", nil
	}
	link, _ = res.Subtitle.Link.(string)
	if link == "" {
		return "", "", nil
	}
	title = "Unknown"
	var t map[string]any
	if json.Unmarshal(res.Title, &t) == nil && t != nil {
		if n, ok := t["title_name"]; ok {
			title = fmt.Sprint(n)
		}
	}
	return link, strings.TrimSpace(title), nil
}

// Download implements provider.Provider: id is the subtitle's link.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	if id == "" {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "no download URL available")
	}
	r, err := p.s.Get(ctx, id, nil)
	if err != nil {
		return provider.File{}, fmt.Errorf("download: %w", err)
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "empty subtitle content")
	}
	return provider.File{
		Data: r.Body, Format: archive.Detect(r.Body),
		Language: provider.Language{Tag: language},
	}, nil
}

// String names the provider in logs.
func (p *Provider) String() string { return Name }
