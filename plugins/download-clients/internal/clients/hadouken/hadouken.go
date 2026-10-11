package hadouken

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are Hadouken's settings; the category is the label torrents
// are added with.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
}

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := client.ValidateURLBase("url base", s.URLBase, "/hadouken"); err != nil {
		return err
	}
	switch {
	case s.Username == "":
		return errors.New("username must not be empty")
	case s.Password == "":
		return errors.New("password must not be empty")
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(7070, "")
	f[len(f)-1].HelpText = "Adds a prefix to the Hadouken url, such as http://[host]:[port]/[urlBase]/api"
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	return append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, false)...)
}

// Definitions returns the definition of Hadouken.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "hadouken", Name: "Hadouken", Protocol: client.Torrent, Fields: fields(),
		Description: "Hadouken, over its JSON-RPC API",
		New:         newHadouken,
	}}
}

func newHadouken(raw []byte, env client.Env) (client.Client, error) {
	s := settings{
		Server:     client.Server{Host: "localhost", Port: 7070},
		Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
	}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	h := &hadouken{settings: s, env: env}
	h.proxy = &proxy{http: env.HTTP, url: s.BaseURL() + "/api", username: s.Username, password: s.Password, log: env.Log}
	return h, nil
}

// hadouken is the client of Radarr's Hadouken.
type hadouken struct {
	settings settings
	proxy    *proxy
	env      client.Env
}

// Host implements client.Client.
func (h *hadouken) Host() string { return h.settings.Host }

// Items implements client.Client: the torrents of either kind's label, or
// all of them for a kind without one.
func (h *hadouken) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := h.proxy.torrents(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for i := range torrents {
		t := &torrents[i]
		kind, ok := h.settings.Match(func(c string) bool { return c == "" || t.Label == c })
		if !ok {
			continue
		}
		var eta time.Duration
		if t.DownloadRate > 0 && t.TotalSize > 0 {
			eta = seconds(float64(t.TotalSize) / float64(t.DownloadRate))
		}
		var ratio float64
		if t.DownloadedBytes > 0 {
			ratio = float64(t.UploadedBytes) / float64(t.DownloadedBytes)
		}
		category := h.settings.Category(kind)
		if kind == 0 && category == "" {
			category = h.settings.TVCategory
		}
		item := client.Item{
			ID: strings.ToUpper(t.InfoHash), Title: t.Name, Category: category, Kind: kind,
			OutputPath: client.JoinRemote(t.SavePath, t.Name), TotalSize: t.TotalSize, RemainingSize: t.TotalSize - t.DownloadedBytes,
			RemainingTime: &eta, SeedRatio: &ratio,
		}
		switch {
		case t.Error != "":
			item.Status = client.Warning
			item.Message = t.Error
		case t.IsFinished && t.State != stateCheckingFiles:
			item.Status = client.Completed
		case t.State == stateQueuedForChecking:
			item.Status = client.Queued
		case t.State == statePaused:
			item.Status = client.Paused
		default:
			item.Status = client.Downloading
		}
		item.CanBeRemoved = opts.RemoveCompletedDownloads && t.IsFinished && t.State == statePaused
		item.CanMoveFiles = item.CanBeRemoved
		items = append(items, item)
	}
	return items, nil
}

// seconds converts seconds to a duration, the longest where it overflows.
func seconds(s float64) time.Duration {
	if d := s * float64(time.Second); d >= math.MaxInt64 {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(s * float64(time.Second))
}

// Remove implements client.Client.
func (h *hadouken) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return h.proxy.remove(ctx, item.ID, deleteData)
}

// Info implements client.Client: torrents of both kinds are saved to
// Hadouken's default folder.
func (h *hadouken) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: h.settings.IsLocalhost()}
	config, err := h.proxy.config(ctx)
	if err != nil {
		return info, err
	}
	if dir, _ := config["bittorrent.defaultSavePath"].(string); strings.TrimSpace(dir) != "" {
		info.OutputRootFolders = []string{dir}
	}
	return info, nil
}

// Add implements client.Client.
func (h *hadouken) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, h.env.Log, h, r)
}

// AddMagnet implements client.TorrentAdder.
func (h *hadouken) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	if err := h.proxy.addURL(ctx, link, h.settings.Category(r.Kind)); err != nil {
		return "", err
	}
	return hash, nil
}

// AddTorrentFile implements client.TorrentAdder.
func (h *hadouken) AddTorrentFile(ctx context.Context, r *client.Release, _, _ string, data []byte) (string, error) {
	hash, err := h.proxy.addFile(ctx, data, h.settings.Category(r.Kind))
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hash), nil
}

// Test implements client.Client.
func (h *hadouken) Test(ctx context.Context) error {
	if err := h.testConnection(ctx); err != nil {
		return err
	}
	if _, err := h.proxy.torrents(ctx); err != nil {
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return nil
}

func (h *hadouken) testConnection(ctx context.Context) error {
	info, err := h.proxy.systemInfo(ctx)
	if err == nil {
		var v client.Version
		if v, err = client.ParseVersion(info.Versions["hadouken"]); err == nil {
			if !v[:2].AtLeast(5, 1) {
				return fmt.Errorf("hadouken version should be at least 5.1; version reported is %s", info.Versions["hadouken"])
			}
			return nil
		}
	}
	var auth *client.AuthError
	if errors.As(err, &auth) {
		return fmt.Errorf("authentication failure: %w", err)
	}
	return fmt.Errorf("unable to connect to Hadouken: %w", err)
}
