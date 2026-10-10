// Package warming keeps the volumes of the media clients are about to
// play awake, as khuaplayer's recent-file warming does: while a client is
// in use, the volumes holding its user's media in progress are read
// every few seconds, and opening an item wakes the volume holding it.
package warming
