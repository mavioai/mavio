// Package newznab searches Usenet indexers through the Newznab API, as
// Prowlarr's Newznab indexer does: the indexer's capabilities are read
// from its caps and kept for a week, and searches use the IDs it takes.
// Torznab, the same API for torrents, reuses the settings, capabilities
// and requests.
package newznab
