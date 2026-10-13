// Package subtitles acquires the subtitles movies and episodes want, as
// Bazarr does: it resolves their subtitle profiles, finds the subtitles
// missing, asks the subtitle provider plugins for them, scores and chooses
// with libs/acquire/subtitles, saves the best beside the videos,
// synchronizes them against the audio where the storage allows it, and
// keeps history, a blocklist and the failed searches of adaptive
// searching. Downloads scored below the maximum are upgraded later.
//
// The work runs as jobs of the job queue: searches of an item, and the
// recurring searches of the wanted movies and episodes and upgrades.
package subtitles
