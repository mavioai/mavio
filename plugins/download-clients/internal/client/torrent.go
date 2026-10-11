package client

import (
	"context"
	"crypto/sha1" //nolint:gosec // BitTorrent v1 info hashes are SHA-1.
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
)

// TorrentAdder adds torrents to a client, as Radarr's TorrentClientBase
// subclasses do; hash is the info hash in upper-case hex, and each returns
// the hash the client knows the torrent by, empty for hash.
type TorrentAdder interface {
	AddMagnet(ctx context.Context, r *Release, hash, link string) (string, error)
	AddTorrentFile(ctx context.Context, r *Release, hash, fileName string, data []byte) (string, error)
}

// AddTorrent sends a release to a torrent client as Radarr's
// TorrentClientBase.Download does: its torrent file when it has one, else
// its magnet link, refusing it with ErrReleaseBlocked when its hash is
// among the release's blocklisted hashes. It returns the download's ID:
// the hash the client returned, in the case it gave it, else the info
// hash in upper case.
func AddTorrent(ctx context.Context, log *slog.Logger, c TorrentAdder, r *Release) (string, error) {
	var hash, actual string
	var err error
	switch {
	case len(r.Data) > 0:
		if hash, err = InfoHash(r.Data); err != nil {
			return "", err
		}
		name := r.FileName
		if name == "" || !strings.HasSuffix(strings.ToLower(name), ".torrent") {
			name = CleanFileName(r.Title) + ".torrent"
		}
		if err := notBlocked(r, hash); err != nil {
			return "", err
		}
		actual, err = c.AddTorrentFile(ctx, r, hash, name, r.Data)
	case r.MagnetURL != "":
		if hash, err = MagnetHash(r.MagnetURL); err != nil {
			return "", err
		}
		if err := notBlocked(r, hash); err != nil {
			return "", err
		}
		actual, err = c.AddMagnet(ctx, r, hash, r.MagnetURL)
	default:
		return "", errors.New("the release has neither a torrent file nor a magnet link")
	}
	if err != nil {
		return "", err
	}
	if actual == "" {
		return hash, nil
	}
	if !strings.EqualFold(actual, hash) && log != nil {
		log.DebugContext(ctx, "the client did not return the expected info hash, the download may be lost track of",
			"release", r.Title, "hash", hash, "returned", actual)
	}
	return actual, nil
}

// notBlocked fails with ErrReleaseBlocked when a torrent's hash is among
// the release's blocklisted hashes, as Radarr's
// EnsureReleaseIsNotBlocklisted.
func notBlocked(r *Release, hash string) error {
	if slices.ContainsFunc(r.BlocklistedHashes, func(h string) bool { return strings.EqualFold(h, hash) }) {
		return fmt.Errorf("%w: %s", ErrReleaseBlocked, r.Title)
	}
	return nil
}

// MagnetHash returns a magnet link's info hash in upper-case hex: the v1
// hash, else the v2 one.
func MagnetHash(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "magnet" {
		return "", fmt.Errorf("not a magnet link: %q", link)
	}
	var v2 string
	for _, xt := range u.Query()["xt"] {
		switch {
		case strings.HasPrefix(strings.ToLower(xt), "urn:btih:"):
			h := xt[len("urn:btih:"):]
			switch len(h) {
			case 40:
				if _, err := hex.DecodeString(h); err == nil {
					return strings.ToUpper(h), nil
				}
			case 32:
				if b, err := base32.StdEncoding.DecodeString(strings.ToUpper(h)); err == nil {
					return strings.ToUpper(hex.EncodeToString(b)), nil
				}
			}
		case strings.HasPrefix(strings.ToLower(xt), "urn:btmh:1220"):
			if h := xt[len("urn:btmh:1220"):]; len(h) == 64 {
				if _, err := hex.DecodeString(h); err == nil {
					v2 = strings.ToUpper(h)
				}
			}
		}
	}
	if v2 != "" {
		return v2, nil
	}
	return "", fmt.Errorf("no info hash in magnet link %q", link)
}

// InfoHash returns a torrent file's info hash in upper-case hex: the
// SHA-1 of its info dictionary, or for torrents of BitTorrent v2 alone the
// SHA-256.
func InfoHash(data []byte) (string, error) {
	d := decoder{data: data}
	info, v2only, err := d.info()
	if err != nil {
		return "", fmt.Errorf("read torrent file: %w", err)
	}
	if v2only {
		sum := sha256.Sum256(info)
		return strings.ToUpper(hex.EncodeToString(sum[:])), nil
	}
	sum := sha1.Sum(info) //nolint:gosec // BitTorrent v1 info hashes are SHA-1.
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

// decoder reads the bencoded values of a torrent file.
type decoder struct {
	data []byte
	pos  int
}

var errBencode = errors.New("malformed bencoding")

// info returns the raw info dictionary of the top-level dictionary, and
// whether it is of BitTorrent v2 alone (meta version 2 without pieces).
func (d *decoder) info() ([]byte, bool, error) {
	if d.pos >= len(d.data) || d.data[d.pos] != 'd' {
		return nil, false, errBencode
	}
	d.pos++
	for d.pos < len(d.data) && d.data[d.pos] != 'e' {
		key, err := d.str()
		if err != nil {
			return nil, false, err
		}
		start := d.pos
		if err := d.skip(0); err != nil {
			return nil, false, err
		}
		if string(key) == "info" {
			raw := d.data[start:d.pos]
			return raw, isV2Only(raw), nil
		}
	}
	return nil, false, errors.New("no info dictionary")
}

// isV2Only reports whether an info dictionary has meta version 2 and no
// v1 pieces.
func isV2Only(info []byte) bool {
	d := decoder{data: info, pos: 1}
	version, pieces := 0, false
	for d.pos < len(d.data) && d.data[d.pos] != 'e' {
		key, err := d.str()
		if err != nil {
			return false
		}
		start := d.pos
		if err := d.skip(0); err != nil {
			return false
		}
		switch string(key) {
		case "meta version":
			if string(d.data[start:d.pos]) == "i2e" {
				version = 2
			}
		case "pieces":
			pieces = true
		}
	}
	return version == 2 && !pieces
}

func (d *decoder) str() ([]byte, error) {
	n := 0
	for d.pos < len(d.data) && d.data[d.pos] != ':' {
		c := d.data[d.pos]
		if c < '0' || c > '9' || n > len(d.data) {
			return nil, errBencode
		}
		n = n*10 + int(c-'0')
		d.pos++
	}
	d.pos++
	if d.pos > len(d.data) || n > len(d.data)-d.pos {
		return nil, errBencode
	}
	s := d.data[d.pos : d.pos+n]
	d.pos += n
	return s, nil
}

// skip passes over one value; depth bounds nesting.
func (d *decoder) skip(depth int) error {
	if depth > 64 || d.pos >= len(d.data) {
		return errBencode
	}
	switch c := d.data[d.pos]; {
	case c == 'i':
		end := d.pos + 1
		for end < len(d.data) && d.data[end] != 'e' {
			end++
		}
		if end >= len(d.data) {
			return errBencode
		}
		d.pos = end + 1
	case c == 'l' || c == 'd':
		d.pos++
		for d.pos < len(d.data) && d.data[d.pos] != 'e' {
			if c == 'd' {
				if _, err := d.str(); err != nil {
					return err
				}
			}
			if err := d.skip(depth + 1); err != nil {
				return err
			}
		}
		if d.pos >= len(d.data) {
			return errBencode
		}
		d.pos++
	case c >= '0' && c <= '9':
		_, err := d.str()
		return err
	default:
		return errBencode
	}
	return nil
}
