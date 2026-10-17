package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Image is what the engine tells of an image.
type Image struct {
	ID          string `json:"Id"`
	RepoDigests []string
	OS          string `json:"Os"`
	Arch        string `json:"Architecture"`
}

// PullProgress is a step of an image pull: a layer's status, such as
// "Downloading" or "Pull complete", with its bytes done of its total when
// known.
type PullProgress struct {
	Layer          string
	Status         string
	Current, Total int64
}

// SplitReference splits an image reference into its repository and its
// digest or tag: "repo:tag@sha256:…" into "repo" and "sha256:…",
// "repo:tag" into "repo" and "tag". A reference with neither has tag
// "latest".
func SplitReference(ref string) (repo, tag string) {
	if name, digest, ok := strings.Cut(ref, "@"); ok {
		repo, _ = splitTag(name)
		return repo, digest
	}
	repo, tag = splitTag(ref)
	if tag == "" {
		tag = "latest"
	}
	return repo, tag
}

// splitTag splits a reference without a digest at its tag's colon, the
// last one after the last slash, so that a registry's port is not a tag.
func splitTag(ref string) (repo, tag string) {
	i := strings.LastIndex(ref, ":")
	if i < 0 || strings.Contains(ref[i:], "/") {
		return ref, ""
	}
	return ref[:i], ref[i+1:]
}

// PullImage pulls an image from its registry, anonymously, calling
// progress, when not nil, with each step the engine reports.
func (c *Client) PullImage(ctx context.Context, ref string, progress func(PullProgress)) error {
	repo, tag := SplitReference(ref)
	resp, err := c.do(ctx, http.MethodPost, "/images/create", url.Values{"fromImage": {repo}, "tag": {tag}}, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	dec := json.NewDecoder(resp.Body)
	for {
		var msg struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("pulling %s: %w", ref, err)
		}
		if msg.Error != "" {
			return fmt.Errorf("pulling %s: %s", ref, msg.Error)
		}
		if progress != nil {
			progress(PullProgress{Layer: msg.ID, Status: msg.Status, Current: msg.ProgressDetail.Current, Total: msg.ProgressDetail.Total})
		}
	}
}

// InspectImage returns an image the engine has, by reference or ID;
// core.ErrNotFound when it has none.
func (c *Client) InspectImage(ctx context.Context, ref string) (Image, error) {
	var img Image
	err := c.call(ctx, http.MethodGet, "/images/"+ref+"/json", nil, nil, &img)
	return img, err
}
