package subtitle

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wlynxg/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
)

// DetectCharset guesses the character set of a text file: from its byte
// order mark, as UTF-8 when it is valid UTF-8, and otherwise statistically
// (legacy code pages such as windows-1253, GBK or Big5). It returns a
// lower-case IANA name such as "utf-8" or "iso-8859-7", or "" when unknown.
func DetectCharset(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return "utf-8"
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE, 0, 0}):
		return "utf-32le"
	case bytes.HasPrefix(data, []byte{0, 0, 0xFE, 0xFF}):
		return "utf-32be"
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return "utf-16le"
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return "utf-16be"
	case utf8.Valid(data):
		return "utf-8"
	}
	return strings.ToLower(chardet.Detect(data).Encoding)
}

// ToUTF8 converts a text file to UTF-8 without a byte order mark,
// detecting its character set with DetectCharset. It returns the detected
// character set; UTF-8 input is returned as is, apart from the byte order
// mark.
func ToUTF8(data []byte) ([]byte, string, error) {
	charset := DetectCharset(data)
	switch charset {
	case "utf-8", "ascii":
		return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}), charset, nil
	case "":
		return nil, "", fmt.Errorf("unknown character set")
	}
	enc, err := decoder(charset)
	if err != nil {
		return nil, charset, err
	}
	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return nil, charset, fmt.Errorf("decode %s: %w", charset, err)
	}
	return bytes.TrimPrefix(out, []byte{0xEF, 0xBB, 0xBF}), charset, nil
}

func decoder(charset string) (encoding.Encoding, error) {
	switch charset {
	case "utf-16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM), nil
	case "utf-16be", "utf-16":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM), nil
	case "utf-32le":
		return utf32.UTF32(utf32.LittleEndian, utf32.UseBOM), nil
	case "utf-32be", "utf-32":
		return utf32.UTF32(utf32.BigEndian, utf32.UseBOM), nil
	case "gb2312", "gbk":
		// GB18030 is a superset; files labeled GB2312 often use GBK.
		return simplifiedchinese.GB18030, nil
	}
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return nil, fmt.Errorf("character set %s: %w", charset, err)
	}
	return enc, nil
}
