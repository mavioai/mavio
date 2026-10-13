// Package subtitles holds Bazarr's rules for acquiring subtitles, as pure
// functions: the subtitles a movie or an episode lacks under its subtitle
// profile, the properties a subtitle's release names share with the video,
// the scores of candidates from subtitle providers and the choice of the
// one to download, the downloads to upgrade, when a language that failed
// is searched again, and the profile that applies to a movie or a series.
//
// Release names are parsed by libs/release in place of guessit; the
// runtime in apps/server describes videos from the server's items and
// asks the subtitle providers.
package subtitles
