// Package release parses release and media file names the way Sonarr and
// Radarr do: series title, season and episode numbers (standard,
// multi-episode, absolute, daily, mini-series, season packs), movie title and
// year, edition, quality (source, resolution, modifier, revision), languages
// and release group.
//
// The rules are ported from Sonarr (series) and Radarr (movies) and keep their
// .NET regular expression syntax, run by github.com/dlclark/regexp2. Where the
// two disagree, the Kind of the release selects the rule set: Movie follows
// Radarr, Series follows Sonarr. Their test cases are ported into
// testdata/cases by tools/testport.
//
// The package holds pure functions over strings; it depends only on the
// standard library and regexp2.
package release
