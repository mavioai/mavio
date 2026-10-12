package downloads

import "testing"

func TestMagnetHash(t *testing.T) {
	tests := []struct{ link, want string }{
		{"magnet:?xt=urn:btih:0123456789ABCDEF0123456789abcdef01234567&dn=x", "0123456789abcdef0123456789abcdef01234567"},
		// Base32: 20 bytes of 0x00 … 0x13.
		{"magnet:?xt=urn:btih:AAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQT", "000102030405060708090a0b0c0d0e0f10111213"},
		{"magnet:?xt=urn:sha1:0123456789abcdef0123456789abcdef01234567", ""},
		{"magnet:?xt=urn:btih:xyz", ""},
		{"https://example.com/a.torrent", ""},
	}
	for _, tt := range tests {
		if got := magnetHash(tt.link); got != tt.want {
			t.Errorf("magnetHash(%q) = %q, want = %q", tt.link, got, tt.want)
		}
	}
}
