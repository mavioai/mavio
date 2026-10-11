// Package releases searches the indexers of the indexer provider plugins
// for the releases of movies and episodes, as Radarr and Sonarr ask
// Prowlarr for them: by the IDs and numbers each indexer can search by,
// then by title when those find nothing, and parses the releases' titles
// with libs/release.
package releases
