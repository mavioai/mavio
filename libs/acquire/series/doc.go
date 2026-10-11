// Package series decides on episode releases as Sonarr's decision
// engine does: it maps releases to a series and its episodes, works out
// their languages and custom formats, runs Sonarr's specifications on
// them, and ranks the ones accepted.
//
// It builds on the root acquire package: candidates embed the root
// Candidate, specifications that Sonarr shares with Radarr run through
// Root, and only the checks that differ for episodes are written here.
package series
