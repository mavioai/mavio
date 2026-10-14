package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"strings"
	"time"
)

// encoding is lowercase base32 without padding, as server IDs and DNS
// labels use.
var encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// Ticket returns the ticket that admits connections to the server with
// an ID through the relay of a region until exp: its expiry in Unix
// seconds and the first 16 bytes of an HMAC-SHA256 under the secret
// mavio.dev shares with the relays, in lowercase base32.
func Ticket(secret []byte, id, region string, exp time.Time) string {
	b := binary.BigEndian.AppendUint32(nil, uint32(exp.Unix()))
	return encoding.EncodeToString(append(b, mac(secret, id, region, b)...))
}

func mac(secret []byte, id, region string, exp []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte("mavio relay ticket\x00" + id + "\x00" + region + "\x00"))
	h.Write(exp)
	return h.Sum(nil)[:16]
}

// CheckTicket reports whether a ticket admits a connection to the server
// with an ID through the relay of a region at now.
func CheckTicket(secret []byte, ticket, id, region string, now time.Time) bool {
	b, err := encoding.DecodeString(ticket)
	if err != nil || len(b) != 20 {
		return false
	}
	if int64(binary.BigEndian.Uint32(b)) < now.Unix() {
		return false
	}
	return hmac.Equal(b[4:], mac(secret, id, region, b[:4]))
}

// ParseName splits a relay name, relay-<region>-<ticket>.<ID>.<zone>.
func ParseName(name, zone string) (region, ticket, id string, ok bool) {
	rest, ok := strings.CutSuffix(strings.ToLower(strings.TrimSuffix(name, ".")), "."+zone)
	if !ok {
		return "", "", "", false
	}
	label, id, ok := strings.Cut(rest, ".")
	if !ok || strings.Contains(id, ".") {
		return "", "", "", false
	}
	label, ok = strings.CutPrefix(label, "relay-")
	if !ok {
		return "", "", "", false
	}
	region, ticket, ok = strings.Cut(label, "-")
	return region, ticket, id, ok && region != "" && ticket != ""
}
