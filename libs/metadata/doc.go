// Package metadata reads NFO files (Kodi's metadata format, also written by
// Jellyfin, Sonarr and Radarr) into a Result: the item fields, its cast and
// crew, images, the watched state and stream hints. It also finds external
// IDs in text and knows where movie NFO files live. It is a pure library:
// the caller reads the files.
package metadata
