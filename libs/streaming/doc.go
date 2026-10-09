// Package streaming serves media as HLS with fMP4 (CMAF) segments: it
// divides a media source into segments, writes the master and media
// playlists, and produces segments on demand with ffmpeg, seeking when a
// client jumps ahead or back and keeping what was produced for the
// playback session.
package streaming
