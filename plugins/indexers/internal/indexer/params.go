package indexer

import (
	"net/url"
	"strings"
)

// Params are a URL's query parameters in the order they are set, as .NET's
// NameValueCollection keeps them: setting a parameter again replaces its
// value, adding it again appends to its value after a comma.
type Params struct {
	keys   []string
	values map[string]string
}

// Set sets a parameter's value.
func (p *Params) Set(key, value string) {
	if p.values == nil {
		p.values = map[string]string{}
	}
	if _, ok := p.values[key]; !ok {
		p.keys = append(p.keys, key)
	}
	p.values[key] = value
}

// Add adds a value to a parameter's.
func (p *Params) Add(key, value string) {
	if v, ok := p.values[key]; ok {
		p.values[key] = v + "," + value
		return
	}
	p.Set(key, value)
}

// Get returns a parameter's value.
func (p *Params) Get(key string) (string, bool) {
	v, ok := p.values[key]
	return v, ok
}

// Del removes a parameter.
func (p *Params) Del(key string) {
	if _, ok := p.values[key]; !ok {
		return
	}
	delete(p.values, key)
	for i, k := range p.keys {
		if k == key {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			break
		}
	}
}

// Len returns the number of parameters.
func (p *Params) Len() int { return len(p.keys) }

// Encode writes the parameters as Prowlarr's GetQueryString does: keys as
// they are, values encoded as WebUtility.UrlEncode encodes them.
func (p *Params) Encode() string {
	parts := make([]string, 0, len(p.keys))
	for _, k := range p.keys {
		parts = append(parts, k+"="+URLEncode(p.values[k]))
	}
	return strings.Join(parts, "&")
}

// URLEncode encodes text as .NET's WebUtility.UrlEncode does: spaces as
// '+', and upper case escapes of all but letters, digits and "-_.!*()".
func URLEncode(s string) string {
	const digits = "0123456789ABCDEF"
	var sb strings.Builder
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			strings.IndexByte("-_.!*()", c) >= 0:
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

// EscapeDataString escapes text as .NET's Uri.EscapeDataString does: all
// but RFC 3986's unreserved characters, spaces as "%20".
func EscapeDataString(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// AddQueryParam adds a parameter to a URL as Prowlarr's
// HttpUri.AddQueryParam does, both escaped as data.
func AddQueryParam(rawURL, key, value string) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
		if strings.HasSuffix(rawURL, "?") || strings.HasSuffix(rawURL, "&") {
			sep = ""
		}
	}
	return rawURL + sep + EscapeDataString(key) + "=" + EscapeDataString(value)
}

// CombinePath joins a base URL and a path as Prowlarr's HttpUri.CombinePath
// does: with one slash between them.
func CombinePath(base, path string) string {
	if path == "" {
		return base
	}
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// RemoveQueryParam removes a parameter, named ignoring case, from a URL's
// query, as Prowlarr's RemoveQueryParam does.
func RemoveQueryParam(rawURL, name string) string {
	base, query, ok := strings.Cut(rawURL, "?")
	if !ok {
		return rawURL
	}
	query, fragment, hasFragment := strings.Cut(query, "#")
	var kept []string
	for part := range strings.SplitSeq(query, "&") {
		k, _, _ := strings.Cut(part, "=")
		if key, err := url.QueryUnescape(k); err == nil && strings.EqualFold(key, name) || part == "" {
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) > 0 {
		base += "?" + strings.Join(kept, "&")
	}
	if hasFragment {
		base += "#" + fragment
	}
	return base
}
