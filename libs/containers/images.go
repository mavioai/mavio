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

// PullQueued is the Status of the PullProgress, without a layer, telling
// that a pull waits for others to end before it starts.
const PullQueued = "Queued"

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

// PullFraction sums the progress of a pull's layers into the fraction
// downloaded. It is not safe for concurrent use.
type PullFraction struct {
	layers  map[string][2]int64
	percent int
}

// Add records a progress message and returns the fraction downloaded,
// reporting whether it moved by a percent or more since last reported.
func (f *PullFraction) Add(p PullProgress) (float64, bool) {
	if p.Layer == "" {
		return 0, false
	}
	if f.layers == nil {
		f.layers = map[string][2]int64{}
	}
	l := f.layers[p.Layer]
	switch {
	case p.Status == "Downloading" && p.Total > 0:
		l = [2]int64{p.Current, p.Total}
	case p.Status == "Download complete" || p.Status == "Pull complete" || p.Status == "Already exists":
		l[0] = l[1]
	default:
		return 0, false
	}
	f.layers[p.Layer] = l
	var cur, total int64
	for _, l := range f.layers {
		cur, total = cur+l[0], total+l[1]
	}
	if total == 0 {
		return 0, false
	}
	frac := float64(cur) / float64(total)
	if pct := int(frac * 100); pct != f.percent {
		f.percent = pct
		return frac, true
	}
	return frac, false
}
