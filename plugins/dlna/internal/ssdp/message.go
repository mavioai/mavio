package ssdp

import (
	"bytes"
	"fmt"
	"net/http"
	"net/textproto"
	"slices"
	"strconv"
	"strings"
)

// Message is an SSDP message: a NOTIFY or M-SEARCH request, or a response
// to a search.
type Message struct {
	// Method is "NOTIFY" or "M-SEARCH"; empty for a response.
	Method string
	// Status is a response's status code.
	Status int
	Header http.Header
}

// Parse reads a datagram. It tolerates bare line feeds and a missing
// final empty line, as devices send them.
func Parse(b []byte) (Message, error) {
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	start := strings.Fields(lines[0])
	var m Message
	switch {
	case len(start) >= 2 && strings.HasPrefix(start[0], "HTTP/"):
		status, err := strconv.Atoi(start[1])
		if err != nil {
			return m, fmt.Errorf("status line %q", lines[0])
		}
		m.Status = status
	case len(start) == 3 && start[1] == "*" && strings.HasPrefix(start[2], "HTTP/"):
		m.Method = start[0]
	default:
		return m, fmt.Errorf("start line %q", lines[0])
	}
	m.Header = http.Header{}
	for _, l := range lines[1:] {
		if l == "" {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			return m, fmt.Errorf("header line %q", l)
		}
		m.Header.Add(textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(k)), strings.TrimSpace(v))
	}
	return m, nil
}

// headerOrder is the order headers are written in, the others following
// by name; devices are known to be picky.
var headerOrder = []string{"Host", "Cache-Control", "Date", "Ext", "Location", "Man", "Mx", "Nt", "Nts", "Server", "St", "Usn"}

// Bytes encodes the message.
func (m Message) Bytes() []byte {
	var b bytes.Buffer
	if m.Method != "" {
		fmt.Fprintf(&b, "%s * HTTP/1.1\r\n", m.Method)
	} else {
		fmt.Fprintf(&b, "HTTP/1.1 %d %s\r\n", m.Status, http.StatusText(m.Status))
	}
	written := map[string]bool{}
	write := func(k string) {
		for _, v := range m.Header[k] {
			b.WriteString(strings.ToUpper(k) + ":")
			if v != "" {
				b.WriteString(" " + v)
			}
			b.WriteString("\r\n")
		}
		written[k] = true
	}
	for _, k := range headerOrder {
		write(k)
	}
	keys := make([]string, 0, len(m.Header))
	for k := range m.Header {
		if !written[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		write(k)
	}
	b.WriteString("\r\n")
	return b.Bytes()
}
