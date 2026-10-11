package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Seed is the seeding a release's indexer asks for, as Radarr's
// TorrentSeedConfiguration; nil for the client's own limits.
type Seed struct {
	Ratio *float64       `json:"ratio,omitempty"`
	Time  *time.Duration `json:"time,omitempty"`
}

// Seeds keeps the seed limits of torrents sent to the clients that cannot
// keep them themselves (Flood, rqbit and rTorrent), by client and info
// hash, saved to a file when one is given so that they hold across
// restarts. Radarr's DownloadSeedConfigProvider finds them again from the
// grab history; the plugin keeps them in its data folder instead.
type Seeds struct {
	mu   sync.Mutex
	file string
	m    map[string]map[string]Seed
}

// NewSeeds returns the seed limits saved in file, or none when the file
// is "" or missing.
func NewSeeds(file string) (*Seeds, error) {
	s := &Seeds{file: file, m: map[string]map[string]Seed{}}
	if file == "" {
		return s, nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.m); err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return s, nil
}

// For returns the seed limits of a client.
func (s *Seeds) For(clientID string) *ClientSeeds { return &ClientSeeds{s: s, id: clientID} }

// Remove forgets the seed limits of clients not in keep.
func (s *Seeds) Remove(keep func(clientID string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.m)
	maps.DeleteFunc(s.m, func(id string, _ map[string]Seed) bool { return !keep(id) })
	if len(s.m) != n {
		s.save()
	}
}

func (s *Seeds) save() {
	if s.file != "" {
		writeJSON(s.file, s.m)
	}
}

// ClientSeeds are the seed limits of one client's torrents.
type ClientSeeds struct {
	s  *Seeds
	id string
}

// Remember keeps the seeding a release asks for, if any, by info hash.
func (c *ClientSeeds) Remember(hash string, r *Release) {
	if r.SeedRatio == nil && r.SeedTime == nil {
		return
	}
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	m := c.s.m[c.id]
	if m == nil {
		m = map[string]Seed{}
		c.s.m[c.id] = m
	}
	m[strings.ToUpper(hash)] = Seed{Ratio: r.SeedRatio, Time: r.SeedTime}
	c.s.save()
}

// Get returns the seeding a torrent's release asked for.
func (c *ClientSeeds) Get(hash string) (Seed, bool) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	seed, ok := c.s.m[c.id][strings.ToUpper(hash)]
	return seed, ok
}

// Forget drops the seed limits of torrents no longer in the client, those
// for which present is false.
func (c *ClientSeeds) Forget(present func(hash string) bool) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	m := c.s.m[c.id]
	n := len(m)
	maps.DeleteFunc(m, func(hash string, _ Seed) bool { return !present(hash) })
	if len(m) == n {
		return
	}
	if len(m) == 0 {
		delete(c.s.m, c.id)
	}
	c.s.save()
}

// writeJSON writes v to file through a temporary file; failures only
// lose what it holds on restart.
func writeJSON(file string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, file)
}
