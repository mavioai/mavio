// Package acquire is the acquisition engine shared by movies and series,
// ported from Radarr and Sonarr: qualities and their definitions, quality
// profiles, custom formats and delay profiles. Releases are parsed by
// libs/release; what acquire decides about them reaches storage, indexers
// and download clients only through ports that apps/server implements.
package acquire
