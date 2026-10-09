package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// errNotFound is fanart.tv's answer for IDs it has no images of.
var errNotFound = errors.New("fanart.tv: not found")

// client calls fanart.tv's API.
type client struct {
	http      *http.Client
	baseURL   string
	key       string
	clientKey string
}

// images fetches the images of an entity, such as "movies/603", as image
// lists by field name.
func (c *client) images(ctx context.Context, path string) (map[string]json.RawMessage, error) {
	q := url.Values{"api_key": {c.key}}
	if c.clientKey != "" {
		q.Set("client_key", c.clientKey)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+path+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// Do's error carries the URL, and with it the API key.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, fmt.Errorf("fanart.tv %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, errNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("fanart.tv %s: %s", path, resp.Status)
	}
	var out map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("fanart.tv %s: %w", path, err)
	}
	return out, nil
}

// fanartImage is one image of a list.
type fanartImage struct {
	URL    string `json:"url"`
	Lang   string `json:"lang"`
	Likes  string `json:"likes"`
	Season string `json:"season"`
}

// list decodes an image list; other fields decode to none.
func list(fields map[string]json.RawMessage, name string) []fanartImage {
	var out []fanartImage
	_ = json.Unmarshal(fields[name], &out)
	return out
}

// kinds are the image lists of each kind of entity, best list first.
var (
	movieKinds = []field{
		{"movieposter", pluginv1.ImageKind_IMAGE_KIND_PRIMARY},
		{"moviebackground", pluginv1.ImageKind_IMAGE_KIND_BACKDROP},
		{"hdmovielogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"movielogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"hdmovieclearart", pluginv1.ImageKind_IMAGE_KIND_ART},
		{"movieart", pluginv1.ImageKind_IMAGE_KIND_ART},
		{"moviedisc", pluginv1.ImageKind_IMAGE_KIND_DISC},
		{"moviebanner", pluginv1.ImageKind_IMAGE_KIND_BANNER},
		{"moviethumb", pluginv1.ImageKind_IMAGE_KIND_THUMB},
	}
	seriesKinds = []field{
		{"tvposter", pluginv1.ImageKind_IMAGE_KIND_PRIMARY},
		{"showbackground", pluginv1.ImageKind_IMAGE_KIND_BACKDROP},
		{"hdtvlogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"clearlogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"hdclearart", pluginv1.ImageKind_IMAGE_KIND_ART},
		{"clearart", pluginv1.ImageKind_IMAGE_KIND_ART},
		{"tvbanner", pluginv1.ImageKind_IMAGE_KIND_BANNER},
		{"tvthumb", pluginv1.ImageKind_IMAGE_KIND_THUMB},
	}
	seasonKinds = []field{
		{"seasonposter", pluginv1.ImageKind_IMAGE_KIND_PRIMARY},
		{"seasonbanner", pluginv1.ImageKind_IMAGE_KIND_BANNER},
		{"seasonthumb", pluginv1.ImageKind_IMAGE_KIND_THUMB},
	}
	artistKinds = []field{
		{"artistthumb", pluginv1.ImageKind_IMAGE_KIND_PRIMARY},
		{"artistbackground", pluginv1.ImageKind_IMAGE_KIND_BACKDROP},
		{"hdmusiclogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"musiclogo", pluginv1.ImageKind_IMAGE_KIND_LOGO},
		{"musicbanner", pluginv1.ImageKind_IMAGE_KIND_BANNER},
	}
	albumKinds = []field{
		{"albumcover", pluginv1.ImageKind_IMAGE_KIND_PRIMARY},
		{"cdart", pluginv1.ImageKind_IMAGE_KIND_DISC},
	}
)

type field struct {
	name string
	kind pluginv1.ImageKind
}

// collect converts the image lists of fields, keeping those that match
// keep, and orders each kind's images: those in language first, then those
// without text, then English ones, then the rest, the most liked first
// within each.
func collect(fields map[string]json.RawMessage, kinds []field, language string, keep func(fanartImage) bool) []*pluginv1.RemoteImage {
	lang, _, _ := strings.Cut(strings.ToLower(language), "-")
	rank := func(l string) int {
		switch l {
		case lang:
			return 0
		case "", "00":
			return 1
		case "en":
			return 2
		}
		return 3
	}
	var out []*pluginv1.RemoteImage
	for _, k := range kinds {
		images := slices.DeleteFunc(list(fields, k.name), func(img fanartImage) bool { return img.URL == "" || keep != nil && !keep(img) })
		slices.SortStableFunc(images, func(a, b fanartImage) int {
			return cmp.Or(cmp.Compare(rank(a.Lang), rank(b.Lang)), cmp.Compare(likes(b), likes(a)))
		})
		for _, img := range images {
			lang := img.Lang
			if lang == "00" {
				lang = ""
			}
			out = append(out, pluginv1.RemoteImage_builder{
				Kind: k.kind.Enum(), Url: proto.String(img.URL), Language: proto.String(lang), Score: proto.Float64(float64(likes(img))),
			}.Build())
		}
	}
	// Kinds listed twice, such as HD and standard logos, keep the order
	// of their lists.
	slices.SortStableFunc(out, func(a, b *pluginv1.RemoteImage) int {
		return cmp.Compare(kindOrder(kinds, a.GetKind()), kindOrder(kinds, b.GetKind()))
	})
	return out
}

func kindOrder(kinds []field, k pluginv1.ImageKind) int {
	return slices.IndexFunc(kinds, func(f field) bool { return f.kind == k })
}

func likes(img fanartImage) int {
	n, _ := strconv.Atoi(img.Likes)
	return n
}
