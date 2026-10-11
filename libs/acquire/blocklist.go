package acquire

import (
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Blocklisted reports whether a release is one of the blocklist entries
// of its movie or series, as Radarr's BlocklistService.Blocklisted: a
// torrent by its info hash, else by its title and indexer; an NZB by its
// title and publish date, or by its title, publish date and size when it
// came from another indexer.
func Blocklisted(entries []core.BlocklistEntry, r *Release) bool {
	if r.Protocol == ProtocolTorrent {
		for i := range entries {
			b := &entries[i]
			switch {
			case r.InfoHash != "":
				if b.TorrentInfoHash != "" && containsFold(b.TorrentInfoHash, r.InfoHash) && sameTorrent(b, r) {
					return true
				}
			case b.Protocol == core.ProtocolTorrent && containsFold(b.SourceTitle, r.Title) && sameTorrent(b, r):
				return true
			}
		}
		return false
	}
	for i := range entries {
		b := &entries[i]
		if b.Protocol == core.ProtocolUsenet && containsFold(b.SourceTitle, r.Title) && sameNZB(b, r) {
			return true
		}
	}
	return false
}

// sameNZB is Radarr's ReleaseComparer.SameNzb.
func sameNZB(b *core.BlocklistEntry, r *Release) bool {
	if b.PublishedDate.Equal(r.PublishDate) {
		return true
	}
	return !sameIndexer(b, r.Indexer) && samePublishedDate(b, r.PublishDate) && sameSize(b, r.Size)
}

// sameTorrent is Radarr's ReleaseComparer.SameTorrent.
func sameTorrent(b *core.BlocklistEntry, r *Release) bool {
	if r.InfoHash != "" {
		return strings.EqualFold(r.InfoHash, b.TorrentInfoHash)
	}
	return sameIndexer(b, r.Indexer)
}

func sameIndexer(b *core.BlocklistEntry, indexer string) bool {
	return strings.TrimSpace(b.Indexer) == "" || strings.EqualFold(b.Indexer, indexer)
}

func samePublishedDate(b *core.BlocklistEntry, published time.Time) bool {
	if b.PublishedDate.IsZero() {
		return true
	}
	return !published.Before(b.PublishedDate.Add(-2*time.Minute)) && !published.After(b.PublishedDate.Add(2*time.Minute))
}

func sameSize(b *core.BlocklistEntry, size int64) bool {
	if b.Size == 0 {
		return true
	}
	d := b.Size - size
	if d < 0 {
		d = -d
	}
	return d <= 2<<20
}

// containsFold reports whether s contains substr, ignoring case, as the
// LIKE queries of Radarr's BlocklistRepository.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
