package core

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ID identifies an entity. It is a UUIDv7 (RFC 9562): IDs created later sort
// after IDs created earlier, which keeps database indexes append-friendly.
type ID [16]byte

// NilID is the zero ID.
var NilID ID

var idGen struct {
	sync.Mutex
	lastMs  int64
	counter uint16 // 12-bit sequence within the same millisecond
}

// NewID returns a new UUIDv7. IDs from one process are strictly increasing,
// even within the same millisecond.
func NewID() ID {
	ms := time.Now().UnixMilli()

	idGen.Lock()
	switch {
	case ms > idGen.lastMs:
		idGen.lastMs = ms
		var b [2]byte
		_, _ = rand.Read(b[:])
		idGen.counter = binary.BigEndian.Uint16(b[:]) & 0x03ff // leave headroom for increments
	case idGen.counter < 0x0fff:
		idGen.counter++
	default:
		// Sequence exhausted: borrow the next millisecond.
		idGen.lastMs++
		idGen.counter = 0
	}
	ms, seq := idGen.lastMs, idGen.counter
	idGen.Unlock()

	var id ID
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)
	id[6] = 0x70 | byte(seq>>8) // version 7
	id[7] = byte(seq)
	_, _ = rand.Read(id[8:])
	id[8] = id[8]&0x3f | 0x80 // RFC 9562 variant
	return id
}

// ParseID parses the canonical form "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return id, fmt.Errorf("%w: invalid ID %q", ErrInvalid, s)
	}
	hexStr := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:36]
	if _, err := hex.Decode(id[:], []byte(hexStr)); err != nil {
		return NilID, fmt.Errorf("%w: invalid ID %q", ErrInvalid, s)
	}
	return id, nil
}

// MustParseID is ParseID that panics on error, for constants and tests.
func MustParseID(s string) ID {
	id, err := ParseID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// IsZero reports whether id is the nil ID.
func (id ID) IsZero() bool { return id == NilID }

// Time returns the creation time encoded in a UUIDv7.
func (id ID) Time() time.Time {
	ms := int64(id[0])<<40 | int64(id[1])<<32 | int64(id[2])<<24 | int64(id[3])<<16 | int64(id[4])<<8 | int64(id[5])
	return time.UnixMilli(ms)
}

// String returns the canonical form.
func (id ID) String() string {
	var b [36]byte
	hex.Encode(b[0:8], id[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], id[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], id[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], id[8:10])
	b[23] = '-'
	hex.Encode(b[24:36], id[10:16])
	return string(b[:])
}

// MarshalText implements encoding.TextMarshaler.
func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (id *ID) UnmarshalText(b []byte) error {
	parsed, err := ParseID(string(b))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
