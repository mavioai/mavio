package flood

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are Flood's settings. The tags take the place of categories:
// a download is of a kind when it has all of the kind's tags.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	// Directories are Radarr's Destination, by kind.
	client.Directories
	MovieTags           []string `json:"movie_tags"`
	TVTags              []string `json:"tv_tags"`
	MoviePostImportTags []string `json:"movie_post_import_tags"`
	TVPostImportTags    []string `json:"tv_post_import_tags"`
	StartOnAdd          bool     `json:"start_on_add"`
	// AdditionalTags are the tags taken from a release's media and
	// details, as Radarr's and Sonarr's AdditionalTags.
	AdditionalTags []string `json:"additional_tags"`
}

// The additional tags: Radarr's AdditionalTags, with Sonarr's TitleSlug
// and Network, the latter being the studio of series.
const (
	tagCollection   = "collection"
	tagTitleSlug    = "title_slug"
	tagQuality      = "quality"
	tagLanguages    = "languages"
	tagReleaseGroup = "release_group"
	tagYear         = "year"
	tagIndexer      = "indexer"
	tagStudio       = "studio"
)

// additionalTags are the additional tags in the order they are offered.
func additionalTags() []string {
	return []string{tagCollection, tagTitleSlug, tagQuality, tagLanguages, tagReleaseGroup, tagYear, tagIndexer, tagStudio}
}

func (s *settings) tags(k client.Kind) []string {
	if k == client.Series {
		return s.TVTags
	}
	return s.MovieTags
}

func (s *settings) postImportTags(k client.Kind) []string {
	if k == client.Series {
		return s.TVPostImportTags
	}
	return s.MoviePostImportTags
}

func fields() []client.Field {
	f := client.ServerFields(3000, "")
	f[len(f)-1].HelpText = "Adds a prefix to the Flood API, such as [protocol]://[host]:[port]/[urlBase]/api"
	f[len(f)-1].Advanced = false
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.DirectoryFields("Flood")...)
	tags := "Initial tags of a download. To be recognized, a download must have all initial tags. This avoids conflicts with unrelated downloads."
	post := "Appends tags after a download is imported."
	return append(f,
		client.Field{Name: "movie_tags", Label: "Movie Tags", Type: client.FieldTag, Default: []string{client.DefaultMovieCategory}, HelpText: tags},
		client.Field{Name: "tv_tags", Label: "Series Tags", Type: client.FieldTag, Default: []string{client.DefaultTVCategory}, HelpText: tags},
		client.Field{Name: "movie_post_import_tags", Label: "Post-Import Movie Tags", Type: client.FieldTag, Advanced: true, HelpText: post},
		client.Field{Name: "tv_post_import_tags", Label: "Post-Import Series Tags", Type: client.FieldTag, Advanced: true, HelpText: post},
		client.Field{
			Name: "additional_tags", Label: "Additional Tags", Type: client.FieldMultiSelect, Advanced: true,
			HelpText: "Adds properties of media as tags. Hints are examples.",
			Options: []client.Option{
				{Value: tagCollection, Label: "Collection (Big Buck Bunny Series)"},
				{Value: tagTitleSlug, Label: "Series Title Slug (the-series)"},
				{Value: tagQuality, Label: "Quality (Bluray-2160p)"},
				{Value: tagLanguages, Label: "Languages (English)"},
				{Value: tagReleaseGroup, Label: "Release Group (Example-Raws)"},
				{Value: tagYear, Label: "Year (2020)"},
				{Value: tagIndexer, Label: "Indexer (Torznab)"},
				{Value: tagStudio, Label: "Studio or Network (C-SPAN)"},
			},
		},
		client.Field{Name: "start_on_add", Label: "Start on Add", Type: client.FieldCheckbox, Default: true},
	)
}

// Definitions returns the definition of Flood.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "flood", Name: "Flood", Protocol: client.Torrent, Fields: fields(),
		Description: "Flood, over its API. Mavio removes torrents by the seed criteria of their indexers.",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{
				Server:     client.Server{Host: "localhost", Port: 3000},
				MovieTags:  []string{client.DefaultMovieCategory},
				TVTags:     []string{client.DefaultTVCategory},
				StartOnAdd: true,
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.Validate(); err != nil {
				return nil, err
			}
			for _, t := range s.AdditionalTags {
				if !slices.Contains(additionalTags(), t) {
					return nil, fmt.Errorf("additional tags: unexpected additional tag %q", t)
				}
			}
			f := &flood{settings: s, env: env, seeds: env.SeedStore()}
			f.proxy = &proxy{http: env.HTTP, settings: &f.settings}
			return f, nil
		},
	}}
}

// flood is the client of Radarr's Flood.
type flood struct {
	settings settings
	proxy    *proxy
	env      client.Env

	// seeds are the seeding of the releases added, by info hash, which
	// Radarr's DownloadSeedConfigProvider finds from the grab history.
	seeds *client.ClientSeeds
}

// Host implements client.Client.
func (f *flood) Host() string { return f.settings.Host }

// Add implements client.Client.
func (f *flood) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, f.env.Log, f, r)
}

// AddMagnet implements client.TorrentAdder.
func (f *flood) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	if err := f.proxy.addURL(ctx, link, f.addTags(r), f.settings.Directory(r.Kind)); err != nil {
		return "", err
	}
	f.seeds.Remember(hash, r)
	return hash, nil
}

// AddTorrentFile implements client.TorrentAdder.
func (f *flood) AddTorrentFile(ctx context.Context, r *client.Release, hash, _ string, data []byte) (string, error) {
	if err := f.proxy.addFile(ctx, base64.StdEncoding.EncodeToString(data), f.addTags(r), f.settings.Directory(r.Kind)); err != nil {
		return "", err
	}
	f.seeds.Remember(hash, r)
	return hash, nil
}

// addTags are the tags of a release's kind and its additional tags, as
// Radarr's and Sonarr's HandleTags.
func (f *flood) addTags(r *client.Release) []string {
	var more []string
	d := &r.Details
	for _, t := range f.settings.AdditionalTags {
		switch t {
		case tagCollection:
			more = append(more, d.Collection)
		case tagTitleSlug:
			more = append(more, d.TitleSlug)
		case tagQuality:
			more = append(more, d.Quality)
		case tagLanguages:
			more = append(more, d.Languages...)
		case tagReleaseGroup:
			more = append(more, d.ReleaseGroup)
		case tagYear:
			if d.Year > 0 {
				more = append(more, strconv.Itoa(d.Year))
			}
		case tagIndexer:
			more = append(more, d.Indexer)
		case tagStudio:
			more = append(more, d.Studio)
		}
	}
	return union(f.settings.tags(r.Kind), more)
}

// union returns tags with more appended, without blanks or duplicates.
func union(tags, more []string) []string {
	out := []string{}
	for _, t := range slices.Concat(tags, more) {
		if strings.TrimSpace(t) != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// kindOf returns the kind whose tags a torrent has all of, and whether
// it is of either; zero when it is of both.
func (f *flood) kindOf(t *torrent) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		for _, tag := range f.settings.tags(k) {
			if !slices.Contains(t.Tags, tag) {
				return false
			}
		}
		return true
	}
	movie, series := of(client.Movie), of(client.Series)
	switch {
	case movie && series:
		return 0, true
	case movie:
		return client.Movie, true
	case series:
		return client.Series, true
	}
	return 0, false
}

// Items implements client.Client.
func (f *flood) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := f.proxy.torrents(ctx)
	if err != nil {
		return nil, err
	}
	f.seeds.Forget(func(hash string) bool {
		for h := range torrents {
			if strings.EqualFold(h, hash) {
				return true
			}
		}
		return false
	})
	var items []client.Item
	for _, hash := range slices.Sorted(maps.Keys(torrents)) {
		t := torrents[hash]
		kind, ok := f.kindOf(&t)
		if !ok {
			continue
		}
		ratio := t.Ratio
		item := client.Item{
			ID: hash, Title: t.Name, Kind: kind, OutputPath: t.Directory,
			TotalSize: t.SizeBytes, RemainingSize: t.SizeBytes - t.BytesDone, SeedRatio: &ratio, Message: t.Message,
		}
		if len(t.Tags) > 0 {
			item.Category = t.Tags[0]
		}
		if t.ETA > 0 {
			eta := time.Duration(t.ETA) * time.Second
			item.RemainingTime = &eta
		}
		switch {
		case slices.Contains(t.Status, "seeding") || slices.Contains(t.Status, "complete"):
			item.Status = client.Completed
		case slices.Contains(t.Status, "stopped"):
			item.Status = client.Paused
		case slices.Contains(t.Status, "error"):
			item.Status = client.Warning
		case slices.Contains(t.Status, "downloading"):
			item.Status = client.Downloading
		default:
			item.Status = client.Queued
		}
		if opts.RemoveCompletedDownloads && item.Status == client.Completed {
			item.CanBeRemoved = f.doneSeeding(item.ID, &t)
			item.CanMoveFiles = item.CanBeRemoved
		}
		items = append(items, item)
	}
	return items, nil
}

// doneSeeding reports whether a torrent reached the ratio or, since it
// finished, the seed time its release asked for.
func (f *flood) doneSeeding(hash string, t *torrent) bool {
	s, ok := f.seeds.Get(hash)
	switch {
	case !ok:
		return false
	case s.Ratio != nil && t.Ratio >= *s.Ratio:
		return true
	case t.DateFinished != nil && *t.DateFinished > 0 && s.Time != nil:
		return f.env.Now().Sub(time.Unix(*t.DateFinished, 0)) >= *s.Time
	}
	return false
}

// ImportItem implements client.Importer: the output path becomes the file
// of a single-file torrent, or the folder all the files of a torrent
// share.
func (f *flood) ImportItem(ctx context.Context, item client.Item, _ client.Options) (client.Item, error) {
	paths, err := f.proxy.contentPaths(ctx, item.ID)
	if err != nil {
		return item, err
	}
	switch len(paths) {
	case 0:
		return item, &client.UnavailableError{Err: fmt.Errorf("failed to fetch list of contents of torrent: %s", item.ID)}
	case 1:
		item.OutputPath = client.JoinRemote(item.OutputPath, paths[0])
		return item, nil
	}
	first := func(p string) string {
		parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
		if len(parts) == 0 {
			return ""
		}
		return parts[0]
	}
	base := first(paths[0])
	for _, p := range paths[1:] {
		if first(p) != base {
			// The output path is the torrent's folder already.
			return item, nil
		}
	}
	item.OutputPath = client.JoinRemote(item.OutputPath, base)
	return item, nil
}

// MarkImported implements client.Marker: the post-import tags of the
// item's kind are added to the torrent's.
func (f *flood) MarkImported(ctx context.Context, item client.Item) error {
	kind := item.Kind
	if kind == 0 {
		kind = client.Movie
	}
	post := f.settings.postImportTags(kind)
	if len(post) == 0 {
		return nil
	}
	torrents, err := f.proxy.torrents(ctx)
	if err != nil {
		return err
	}
	for hash, t := range torrents {
		if strings.EqualFold(hash, item.ID) {
			return f.proxy.setTags(ctx, hash, union(t.Tags, post))
		}
	}
	return nil
}

// Remove implements client.Client.
func (f *flood) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if err := f.proxy.remove(ctx, item.ID, deleteData); err != nil {
		return err
	}
	f.seeds.Forget(func(hash string) bool { return !strings.EqualFold(hash, item.ID) })
	return nil
}

// Info implements client.Client.
func (f *flood) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: f.settings.IsLocalhost()}
	var def *string
	for _, k := range client.Kinds {
		dir := f.settings.Directory(k)
		if strings.TrimSpace(dir) == "" {
			if def == nil {
				d, err := f.proxy.defaultDirectory(ctx)
				if err != nil {
					return info, err
				}
				def = &d
			}
			dir = *def
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Test implements client.Client.
func (f *flood) Test(ctx context.Context) error {
	return f.proxy.verify(ctx)
}
