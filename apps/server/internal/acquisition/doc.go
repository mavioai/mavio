// Package acquisition acquires the wanted movies, as Radarr's services
// do: it adds movies by their TMDB IDs, searches the indexers of the
// indexer provider plugins for them in searches and RSS syncs, decides
// on the releases with libs/acquire, sends the best to the download
// clients of the download client provider plugins and keeps the others
// pending, follows the downloads, imports the finished ones into the
// movie libraries, and blocklists and searches again for failed ones.
//
// The work runs as jobs of the job queue: searches, RSS syncs, queue
// checks and recycle bin cleanups, the recurring ones scheduling their
// next run.
package acquisition
