package indexer

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// publicTrackers are the trackers of the magnet links built for public
// sites' info hashes, as Prowlarr's MagnetLinkBuilder lists them.
var publicTrackers = []string{
	"http://tracker.opentrackr.org:1337/announce",
	"udp://tracker.auctor.tv:6969/announce",
	"udp://opentracker.i2p.rocks:6969/announce",
	"https://opentracker.i2p.rocks:443/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://tracker.openbittorrent.com:6969/announce",
	"http://tracker.openbittorrent.com:80/announce",
	"udp://open.stealth.si:80/announce",
	"udp://tracker.torrent.eu.org:451/announce",
	"udp://tracker.moeking.me:6969/announce",
	"udp://explodie.org:6969/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://uploads.gamecoast.net:6969/announce",
	"udp://tracker1.bt.moack.co.kr:80/announce",
	"udp://tracker.tiny-vps.com:6969/announce",
	"udp://tracker.theoks.net:6969/announce",
	"udp://tracker.skyts.net:6969/announce",
	"udp://tracker-udp.gbitt.info:80/announce",
	"udp://open.tracker.ink:6969/announce",
	"udp://movies.zsw.ca:6969/announce",
}

// PublicMagnetLink returns a magnet link of a hex info hash with public
// trackers, written as MonoTorrent's MagnetLink.ToV1String writes it.
func PublicMagnetLink(infoHash, title string) (string, error) {
	b, err := hex.DecodeString(infoHash)
	if err != nil || len(b) != 20 {
		return "", fmt.Errorf("infohash %q is not 40 hex characters", infoHash)
	}
	var sb strings.Builder
	sb.WriteString("magnet:?xt=urn:btih:")
	sb.WriteString(strings.ToUpper(hex.EncodeToString(b)))
	sb.WriteString("&dn=")
	sb.WriteString(monoURLEncode(title))
	for _, tr := range publicTrackers {
		sb.WriteString("&tr=")
		sb.WriteString(monoURLEncode(tr))
	}
	return sb.String(), nil
}

// monoURLEncode encodes UTF-8 text as Mono's HttpUtility.UrlEncode does:
// spaces as '+', lower case hex escapes.
func monoURLEncode(s string) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			strings.IndexByte("-_.!*'()", c) >= 0:
			sb.WriteByte(c)
		case c == ' ':
			sb.WriteByte('+')
		default:
			sb.WriteByte('%')
			sb.WriteByte(digits[c>>4])
			sb.WriteByte(digits[c&0xf])
		}
	}
	return sb.String()
}
