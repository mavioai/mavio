package releases

import (
	"sync"
	"time"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// indexerCacheLife is how long a provider's indexers are kept while it
// keeps its revision.
const indexerCacheLife = time.Hour

// IndexerCache keeps the indexers of each provider, with their
// capabilities, so that searches need not list them first, as Radarr and
// Sonarr keep their indexers' capabilities with the indexers. A
// provider's indexers are listed again when its revision changes, as it
// does when the plugin restarts or is configured, and after an hour. The
// zero value is ready to use and safe for concurrent use.
type IndexerCache struct {
	// Now is the clock; time.Now when nil.
	Now func() time.Time

	mu      sync.Mutex
	entries map[string]indexerEntry
}

// indexerEntry is the indexers a provider listed at a revision.
type indexerEntry struct {
	revision uint64
	listed   time.Time
	indexers []*pluginv1.Indexer
}

func (c *IndexerCache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// get returns the indexers kept for a provider at its revision.
func (c *IndexerCache) get(p Provider) ([]*pluginv1.Indexer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[p.ID]
	if !ok || e.revision != p.Revision || c.now().Sub(e.listed) >= indexerCacheLife {
		return nil, false
	}
	return e.indexers, true
}

// put keeps the indexers a provider listed at its revision.
func (c *IndexerCache) put(p Provider, indexers []*pluginv1.Indexer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]indexerEntry{}
	}
	c.entries[p.ID] = indexerEntry{revision: p.Revision, listed: c.now(), indexers: indexers}
}
