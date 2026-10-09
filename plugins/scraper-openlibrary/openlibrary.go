package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// userAgent identifies the plugin, as Open Library asks.
const userAgent = "Mavio/0.1 (https://github.com/mavioai/mavio)"

// errNotFound is Open Library's answer for unknown keys.
var errNotFound = errors.New("openlibrary: not found")

// client calls Open Library's API.
type client struct {
	http               *http.Client
	baseURL, coversURL string
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("openlibrary %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("openlibrary %s: %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("openlibrary %s: %w", path, err)
	}
	return nil
}

// olDoc is a search result: a work with what its editions share.
type olDoc struct {
	Key              string   `json:"key"` // "/works/OL45804W"
	Title            string   `json:"title"`
	AuthorNames      []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	CoverID          int64    `json:"cover_i"`
	ISBN             []string `json:"isbn"`
	Publishers       []string `json:"publisher"`
	Subjects         []string `json:"subject"`
}

// olText is a text that is a string or {"type": …, "value": …}.
type olText string

func (t *olText) UnmarshalJSON(data []byte) error {
	var s string
	if json.Unmarshal(data, &s) == nil {
		*t = olText(s)
		return nil
	}
	var v struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*t = olText(v.Value)
	return nil
}

type olKey struct {
	Key string `json:"key"`
}

// olWork is a work.
type olWork struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	Subtitle         string   `json:"subtitle"`
	Description      olText   `json:"description"`
	Subjects         []string `json:"subjects"`
	FirstPublishDate string   `json:"first_publish_date"`
	Covers           []int64  `json:"covers"`
	Authors          []struct {
		Author olKey `json:"author"`
	} `json:"authors"`
}

// olEdition is an edition, as an ISBN names it.
type olEdition struct {
	Title       string   `json:"title"`
	Publishers  []string `json:"publishers"`
	PublishDate string   `json:"publish_date"`
	Covers      []int64  `json:"covers"`
	Works       []olKey  `json:"works"`
}

func (c *client) search(ctx context.Context, title, author string, limit int) ([]olDoc, error) {
	q := url.Values{
		"title": {title}, "limit": {strconv.Itoa(limit)},
		"fields": {"key,title,author_name,first_publish_year,cover_i,isbn,publisher,subject"},
	}
	if author != "" {
		q.Set("author", author)
	}
	var out struct {
		Docs []olDoc `json:"docs"`
	}
	return out.Docs, c.get(ctx, "/search.json", q, &out)
}

func (c *client) work(ctx context.Context, id string) (olWork, error) {
	var w olWork
	return w, c.get(ctx, "/works/"+url.PathEscape(id)+".json", nil, &w)
}

func (c *client) edition(ctx context.Context, isbn string) (olEdition, error) {
	var e olEdition
	return e, c.get(ctx, "/isbn/"+url.PathEscape(isbn)+".json", nil, &e)
}

func (c *client) authorName(ctx context.Context, key string) (string, error) {
	var a struct {
		Name string `json:"name"`
	}
	return a.Name, c.get(ctx, key+".json", nil, &a)
}

// coverURL is a cover's large image.
func (c *client) coverURL(id int64) string {
	return c.coversURL + "/b/id/" + strconv.FormatInt(id, 10) + "-L.jpg"
}

// workID returns the ID of a work key such as "/works/OL45804W".
func workID(key string) string {
	return strings.TrimPrefix(key, "/works/")
}
