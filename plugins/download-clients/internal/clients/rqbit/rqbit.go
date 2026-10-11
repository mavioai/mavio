package rqbit

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of rqbit.
type settings struct {
	client.Server
}

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	return client.ValidateURLBase("url base", s.URLBase, "/rqbit")
}

// Definitions returns the definition of rqbit.
func Definitions() []client.Definition {
	f := client.ServerFields(3030, "/")
	f[len(f)-1].HelpText = "Adds a prefix to the rqbit url, such as http://[host]:[port]/[urlBase]"
	return []client.Definition{{
		ID: "rqbit", Name: "RQBit", Protocol: client.Torrent, Fields: f,
		Description: "rqbit, over its HTTP API",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{Server: client.Server{Host: "localhost", Port: 3030, URLBase: "/"}}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.validate(); err != nil {
				return nil, err
			}
			r := &rqbit{settings: s, env: env, seeds: env.SeedStore()}
			r.proxy = &proxy{http: env.HTTP, settings: &r.settings}
			return r, nil
		},
	}}
}

// rqbit is the client of Radarr's RQBit.
type rqbit struct {
	settings settings
	proxy    *proxy
	env      client.Env

	// seeds are the seed configurations of added torrents by hash, as
	// Radarr's DownloadSeedConfigProvider finds them.
	seeds *client.ClientSeeds
}

// Host implements client.Client.
func (r *rqbit) Host() string { return r.settings.Host }

// Add implements client.Client.
func (r *rqbit) Add(ctx context.Context, rel *client.Release) (string, error) {
	id, err := client.AddTorrent(ctx, r.env.Log, r, rel)
	if err != nil {
		return "", err
	}
	r.seeds.Remember(id, rel)
	return id, nil
}

// AddMagnet implements client.TorrentAdder.
func (r *rqbit) AddMagnet(ctx context.Context, _ *client.Release, _, link string) (string, error) {
	return r.proxy.add(ctx, "text/plain", []byte(link))
}

// AddTorrentFile implements client.TorrentAdder.
func (r *rqbit) AddTorrentFile(ctx context.Context, _ *client.Release, _, _ string, data []byte) (string, error) {
	return r.proxy.add(ctx, "application/x-bittorrent", data)
}

// Items implements client.Client.
func (r *rqbit) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := r.proxy.torrents(ctx)
	if err != nil {
		return nil, err
	}
	r.env.Log.DebugContext(ctx, "retrieved metadata of torrents in client", "count", len(torrents))
	r.forget(torrents)
	var items []client.Item
	for i := range torrents {
		t := &torrents[i]
		if strings.TrimSpace(t.Path) == "" {
			r.env.Log.WarnContext(ctx, "torrent has an empty download path and will not be processed", "torrent", t.Name)
			continue
		}
		if strings.HasPrefix(t.Path, ".") {
			r.env.Log.WarnContext(ctx, "torrent has a download path starting with '.' and will not be processed", "torrent", t.Name)
			continue
		}
		ratio := t.Ratio
		remaining := time.Duration(0)
		if t.DownRate > 0 {
			remaining = time.Duration(t.RemainingSize/t.DownRate) * time.Second
		}
		item := client.Item{
			ID: t.Hash, Title: t.Name, Message: t.Message, OutputPath: t.Path,
			TotalSize: t.TotalSize, RemainingSize: t.RemainingSize, RemainingTime: &remaining, SeedRatio: &ratio,
		}
		switch {
		case t.IsFinished:
			item.Status = client.Completed
		case t.HasError:
			item.Status = client.Warning
		case t.IsActive:
			item.Status = client.Downloading
		default:
			item.Status = client.Paused
		}
		if opts.RemoveCompletedDownloads && t.IsFinished {
			if seed, ok := r.seeds.Get(t.Hash); ok {
				// rqbit tells no time of finishing, so the seed time
				// cannot be told to be reached.
				item.CanBeRemoved = seed.Ratio != nil && ratio >= *seed.Ratio
				item.CanMoveFiles = item.CanBeRemoved
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// forget drops the seed configurations of torrents no longer in rqbit.
func (r *rqbit) forget(torrents []torrent) {
	r.seeds.Forget(func(hash string) bool {
		return slices.ContainsFunc(torrents, func(t torrent) bool { return strings.EqualFold(t.Hash, hash) })
	})
}

// Remove implements client.Client: rqbit deletes the files itself.
func (r *rqbit) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return r.proxy.remove(ctx, strings.ToLower(item.ID), deleteData)
}

// Info implements client.Client.
func (r *rqbit) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: r.settings.IsLocalhost()}, nil
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// Test implements client.Client.
func (r *rqbit) Test(ctx context.Context) error {
	s, err := r.proxy.version(ctx)
	if err != nil {
		r.env.Log.ErrorContext(ctx, "failed to determine RQBit version", "err", err)
		return fmt.Errorf("unable to connect to RQBit: %w", err)
	}
	if strings.TrimSpace(s) == "" {
		return errors.New("unable to determine the RQBit version; check that RQBit is running and accessible")
	}
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return fmt.Errorf("unable to parse the RQBit version %q; check that RQBit is running and accessible", s)
	}
	v := make([]int, 3)
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	if slices.Compare(v, []int{8, 0, 0}) < 0 {
		return fmt.Errorf("the RQBit version %d.%d.%d is not supported; upgrade to 8.0.0 or later", v[0], v[1], v[2])
	}
	return nil
}
