package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// userAgent identifies the plugin, as OpenSubtitles requires.
const userAgent = "Mavio v0.1"

// maxSubtitle bounds a downloaded subtitle file.
const maxSubtitle = 10 << 20

// errUnauthorized is the answer to an expired session.
var errUnauthorized = errors.New("opensubtitles: unauthorized")

// client calls the OpenSubtitles.com REST API.
type client struct {
	http    *http.Client
	baseURL string
	key     string
	token   string
}

func (c *client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", c.key)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("opensubtitles %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errUnauthorized
	case resp.StatusCode != http.StatusOK:
		var msg struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&msg)
		return fmt.Errorf("opensubtitles %s: %s: %s", path, resp.Status, msg.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("opensubtitles %s: %w", path, err)
	}
	return nil
}

// login opens a session for an account; it returns its token and the API
// host the account is served by.
func (c *client) login(ctx context.Context, username, password string) (string, string, error) {
	var out struct {
		Token   string `json:"token"`
		BaseURL string `json:"base_url"`
	}
	err := c.do(ctx, http.MethodPost, "/login", nil, map[string]string{"username": username, "password": password}, &out)
	if err != nil {
		return "", "", fmt.Errorf("log in: %w", err)
	}
	return out.Token, out.BaseURL, nil
}

type osSubtitle struct {
	ID         string `json:"id"`
	Attributes struct {
		Language         string  `json:"language"`
		DownloadCount    int     `json:"download_count"`
		HearingImpaired  bool    `json:"hearing_impaired"`
		ForeignPartsOnly bool    `json:"foreign_parts_only"`
		Ratings          float64 `json:"ratings"`
		MovieHashMatch   bool    `json:"moviehash_match"`
		Release          string  `json:"release"`
		Comments         string  `json:"comments"`
		Files            []struct {
			FileID   int64  `json:"file_id"`
			FileName string `json:"file_name"`
		} `json:"files"`
	} `json:"attributes"`
}

func (c *client) search(ctx context.Context, query url.Values) ([]osSubtitle, error) {
	var out struct {
		Data []osSubtitle `json:"data"`
	}
	return out.Data, c.do(ctx, http.MethodGet, "/subtitles", query, nil, &out)
}

// download returns a subtitle file.
func (c *client) download(ctx context.Context, fileID int64) ([]byte, string, error) {
	var link struct {
		Link     string `json:"link"`
		FileName string `json:"file_name"`
	}
	if err := c.do(ctx, http.MethodPost, "/download", nil, map[string]int64{"file_id": fileID}, &link); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.Link, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("opensubtitles download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("opensubtitles download: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSubtitle+1))
	if err != nil {
		return nil, "", fmt.Errorf("opensubtitles download: %w", err)
	}
	if len(data) > maxSubtitle {
		return nil, "", fmt.Errorf("opensubtitles download: larger than %d bytes", maxSubtitle)
	}
	return data, link.FileName, nil
}
