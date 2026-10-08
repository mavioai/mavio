// Package probe runs ffprobe and normalizes its output into a core media
// source and the metadata found in tags, following Jellyfin's
// ProbeResultNormalizer: container names, stream kinds and flags, aspect
// ratios and anamorphism, Dolby Vision and HDR10+ metadata, estimated
// bitrates, chapters, and music and video tags.
package probe
