package cardigann

import (
	"cmp"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dlclark/regexp2"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/unicode/norm"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// Settings are a Cardigann indexer's settings.
type Settings struct {
	// Definition is the ID of the definition, its file name without
	// extension.
	Definition string `json:"definition"`
	// BaseURL replaces the definition's first link.
	BaseURL string `json:"base_url,omitempty"`
	// Fields are the values of the definition's settings by name:
	// strings, booleans for checkboxes, and option keys or indexes for
	// selects. "cookie" is the cookie of definitions logging in by
	// cookie, "CAPTCHA" the text of a captcha.
	Fields map[string]any `json:"fields,omitempty"`
}

// The fields the parser leaves out when they are missing.
var optionalFields = []string{"imdb", "imdbid", "tmdbid", "rageid", "tvdbid", "tvmazeid", "traktid", "doubanid", "poster", "banner", "description", "genre"}

// engine reads a definition, as Prowlarr's CardigannBase does.
type engine struct {
	def      *Definition
	settings Settings
	// encoding is that of the site; nil for UTF-8.
	encoding          encoding.Encoding
	categories        indexer.Categories
	defaultCategories []string
	sel               selectors
	now               func() time.Time
	log               *slog.Logger
}

// newEngine returns the engine of a definition, mapping its categories.
func newEngine(def *Definition, s Settings, now func() time.Time, log *slog.Logger) (*engine, error) {
	e := &engine{def: def, settings: s, now: now, log: log}
	if e.now == nil {
		e.now = time.Now
	}
	if e.log == nil {
		e.log = slog.Default()
	}
	enc, err := htmlindex.Get(def.Encoding)
	if err != nil {
		return nil, fmt.Errorf("definition %s: encoding %s: %w", def.ID, def.Encoding, err)
	}
	if enc != xunicode.UTF8 {
		e.encoding = enc
	}
	for _, c := range def.Caps.Categories {
		cat := categoryByName(c.Value)
		if cat == nil {
			e.log.Error("cardigann: invalid Torznab category", "definition", def.ID, "id", c.Key, "category", c.Value)
			continue
		}
		e.categories.Add(c.Key, cat, "")
	}
	for _, m := range def.Caps.CategoryMappings {
		if m.Cat == "" {
			return nil, fmt.Errorf("definition %s: category mapping %s has no category", def.ID, m.ID)
		}
		cat := categoryByName(m.Cat)
		if cat == nil {
			e.log.Error("cardigann: invalid Torznab category", "definition", def.ID, "id", m.ID, "category", m.Cat)
			continue
		}
		e.categories.Add(m.ID, cat, m.Desc)
		if m.Default {
			e.defaultCategories = append(e.defaultCategories, m.ID)
		}
	}
	return e, nil
}

// categoryByName returns the standard category with the name, such as
// "Movies/HD"; nil for none.
func categoryByName(name string) *indexer.Category {
	for _, c := range indexer.StandardCategories() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// siteLink returns the site's URL: the settings' unless it is a legacy
// link, else the definition's first.
func (e *engine) siteLink() string {
	link := e.def.Links[0]
	if e.settings.BaseURL == "" || slices.Contains(e.def.LegacyLinks, e.settings.BaseURL) {
		return link
	}
	return e.settings.BaseURL
}

// resolvePath resolves a path against a URL, the site's when base is "".
func (e *engine) resolvePath(path, base string) (string, error) {
	if base == "" {
		base = e.siteLink()
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", base, err)
	}
	r, err := url.Parse(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", path, err)
	}
	return absoluteURI(b.ResolveReference(r)), nil
}

// absoluteURI returns a URL as .NET's Uri.AbsoluteUri does, escaping the
// characters URLs may not have in their queries and fragments too.
func absoluteURI(u *url.URL) string {
	u.RawQuery = escapeURIChars(u.RawQuery)
	s := u.String()
	if u.Fragment != "" {
		s = strings.TrimSuffix(s, "#"+u.EscapedFragment()) + "#" + escapeURIChars(u.EscapedFragment())
	}
	return s
}

func escapeURIChars(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c >= 0x7F || strings.IndexByte(`"<>\^`+"`{|}", c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// baseVariables returns the variables of every template: the site's link,
// .True, .False, this year and the settings, as .Config.<name>.
func (e *engine) baseVariables() (Vars, error) {
	vars := Vars{
		".Config.sitelink": e.siteLink(),
		".True":            "True",
		".False":           nil,
		".Today.Year":      strconv.Itoa(e.now().Year()),
	}
	for _, s := range e.def.Settings {
		name := ".Config." + s.Name
		value, set := e.settings.Fields[s.Name]
		switch s.Type {
		case "text", "password":
			if !set {
				value = s.Default
			}
			vars[name] = value
		case "checkbox":
			if !set {
				value = s.Default
			}
			var on bool
			switch v := value.(type) {
			case bool:
				on = v
			case string:
				on, _ = strconv.ParseBool(v)
			}
			if on {
				vars[name] = ".True"
			} else {
				vars[name] = nil
			}
		case "select":
			keys := make([]string, 0, len(s.Options))
			for _, o := range s.Options {
				keys = append(keys, o.Key)
			}
			slices.SortStableFunc(keys, cmp.Compare)
			i := slices.Index(keys, s.Default)
			switch v := value.(type) {
			case string:
				if j := slices.Index(keys, v); j >= 0 {
					i = j
				} else if n, err := strconv.Atoi(v); err == nil {
					i = n
				}
			case float64:
				i = int(v)
			case int:
				i = v
			}
			if i < 0 || i >= len(keys) {
				return nil, fmt.Errorf("setting %s: no option %v", s.Name, value)
			}
			vars[name] = keys[i]
		case "info", "info_cookie", "info_flaresolverr", "info_useragent", "info_category_8000", "cardigannCaptcha":
		default:
			return nil, fmt.Errorf("type %s is not supported", s.Type)
		}
	}
	return vars, nil
}

// headers returns templated headers; each header's first value.
func (e *engine) headers(h Ordered[[]string], vars Vars) (map[string]string, error) {
	out := map[string]string{}
	for _, entry := range h {
		if len(entry.Value) == 0 {
			continue
		}
		v, err := e.applyTemplate(entry.Value[0], vars, nil)
		if err != nil {
			return nil, err
		}
		out[entry.Key] = v
	}
	return out, nil
}

// addURIVariables adds the parts of a URL to the variables, prefixed.
func addURIVariables(vars Vars, link *url.URL, prefix string) {
	vars[prefix+".AbsoluteUri"] = absoluteURI(&url.URL{Scheme: link.Scheme, Opaque: link.Opaque, User: link.User, Host: link.Host, Path: link.Path, RawPath: link.RawPath, RawQuery: link.RawQuery, Fragment: link.Fragment})
	vars[prefix+".AbsolutePath"] = link.EscapedPath()
	vars[prefix+".Scheme"] = link.Scheme
	vars[prefix+".Host"] = link.Hostname()
	port := link.Port()
	if port == "" {
		switch link.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
			port = "-1"
		}
	}
	vars[prefix+".Port"] = port
	query := ""
	if link.RawQuery != "" {
		query = "?" + link.RawQuery
	}
	vars[prefix+".PathAndQuery"] = link.EscapedPath() + query
	vars[prefix+".Query"] = query
	for part := range strings.SplitSeq(link.RawQuery, "&") {
		k, v, _ := strings.Cut(part, "=")
		if k == "" {
			continue
		}
		k, _ = url.QueryUnescape(k)
		v, _ = url.QueryUnescape(v)
		if _, ok := vars[prefix+".Query."+k]; !ok {
			vars[prefix+".Query."+k] = v
		}
	}
}

// dateLayout is how filters write the dates they read, for the date
// field to read.
const dateLayout = time.RFC1123Z

// validateDelimiters split the words of the validate filter and genres.
const validateDelimiters = ", /)(.;[]\"|:"

// applyFilters applies filters to a value, as Prowlarr's ApplyFilters
// does; unknown filters are logged and ignored.
func (e *engine) applyFilters(data string, filters []Filter, vars Vars) (string, error) {
	for _, f := range filters {
		var err error
		if data, err = e.applyFilter(data, &f, vars); err != nil {
			return "", fmt.Errorf("filter %s: %w", f.Name, err)
		}
	}
	return data, nil
}

func (e *engine) applyFilter(data string, f *Filter, vars Vars) (string, error) {
	arg, _ := f.arg()
	switch f.Name {
	case "querystring":
		return parse.QueryArg(data, arg), nil
	case "timeparse", "dateparse":
		t, err := parse.Layout(e.now(), data, arg)
		if err != nil {
			e.log.Debug("cardigann: date not parsed", "definition", e.def.ID, "error", err)
			return data, nil
		}
		return t.Format(dateLayout), nil
	case "regexp":
		re, err := regexp2.Compile(arg, regexp2.None)
		if err != nil {
			return "", err
		}
		m, err := re.FindStringMatch(data)
		if err != nil || m == nil {
			return "", err
		}
		return group(m, 1), nil
	case "re_replace":
		pattern, err := f.listArg(0)
		if err != nil {
			return "", err
		}
		replacement, err := f.listArg(1)
		if err != nil {
			return "", err
		}
		if replacement, err = e.applyTemplate(replacement, vars, nil); err != nil {
			return "", err
		}
		re, err := regexp2.Compile(pattern, regexp2.None)
		if err != nil {
			return "", err
		}
		return re.Replace(data, replacement, -1, -1)
	case "split":
		sep, err := f.listArg(0)
		if err != nil {
			return "", err
		}
		posArg, err := f.listArg(1)
		if err != nil {
			return "", err
		}
		pos, err := strconv.Atoi(posArg)
		if err != nil {
			return "", err
		}
		if sep == "" {
			return "", fmt.Errorf("empty separator")
		}
		r, _ := firstRune(sep)
		parts := strings.Split(data, string(r))
		if pos < 0 {
			pos += len(parts)
		}
		if pos < 0 || pos >= len(parts) {
			return "", fmt.Errorf("index %s out of range", posArg)
		}
		return parts[pos], nil
	case "replace":
		from, err := f.listArg(0)
		if err != nil {
			return "", err
		}
		to, err := f.listArg(1)
		if err != nil {
			return "", err
		}
		if to, err = e.applyTemplate(to, vars, nil); err != nil {
			return "", err
		}
		if from == "" {
			return "", fmt.Errorf("empty string to replace")
		}
		return strings.ReplaceAll(data, from, to), nil
	case "trim":
		if c, ok := firstRune(arg); ok {
			return strings.Trim(data, string(c)), nil
		}
		return strings.TrimSpace(data), nil
	case "prepend":
		s, err := e.applyTemplate(arg, vars, nil)
		return s + data, err
	case "append":
		s, err := e.applyTemplate(arg, vars, nil)
		return data + s, err
	case "tolower":
		return strings.ToLower(data), nil
	case "toupper":
		return strings.ToUpper(data), nil
	case "urldecode":
		return urlDecode(data, e.encoding), nil
	case "urlencode":
		return urlEncode(data, e.encoding), nil
	case "htmldecode":
		return html.UnescapeString(data), nil
	case "htmlencode":
		return htmlEncode(data), nil
	case "timeago", "reltime":
		t, err := parse.TimeAgo(e.now(), data)
		if err != nil {
			return "", err
		}
		return t.Format(dateLayout), nil
	case "fuzzytime":
		t, err := parse.Unknown(e.now(), data, "")
		if err != nil {
			return "", err
		}
		return t.Format(dateLayout), nil
	case "validfilename":
		return validFileName(data), nil
	case "diacritics":
		if arg != "replace" {
			return "", fmt.Errorf("unsupported diacritics filter argument")
		}
		return removeDiacritics(data), nil
	case "jsonjoinarray":
		path, err := f.listArg(0)
		if err != nil {
			return "", err
		}
		sep, err := f.listArg(1)
		if err != nil {
			return "", err
		}
		doc, err := parseJSON(data, e.now().Location())
		if err != nil {
			return "", err
		}
		if _, ok := doc.(*jsonObject); !ok {
			return "", fmt.Errorf("not a JSON object")
		}
		v, ok, err := selectToken(doc, path)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("%s matched nothing", path)
		}
		var parts []string
		for _, c := range children([]any{v}) {
			parts = append(parts, jsonText(c))
		}
		return strings.Join(parts, sep), nil
	case "hexdump":
		var b strings.Builder
		for _, r := range data {
			fmt.Fprintf(&b, "%c(%02X)", r, r)
		}
		e.log.Debug("cardigann: strdump", "definition", e.def.ID, "data", b.String())
		return data, nil
	case "strdump":
		e.log.Debug("cardigann: strdump", "definition", e.def.ID, "tag", arg,
			"data", strings.NewReplacer("\r", `\r`, "\n", `\n`, " ", `\xA0`).Replace(data))
		return data, nil
	case "validate":
		split := func(s string) []string {
			return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return strings.ContainsRune(validateDelimiters, r) })
		}
		have := split(data)
		var valid []string
		for _, w := range split(arg) {
			if w = strings.TrimSpace(w); w != "" && slices.Contains(have, w) && !slices.Contains(valid, w) {
				valid = append(valid, w)
			}
		}
		return strings.Join(valid, ", "), nil
	}
	e.log.Error("cardigann: unsupported field filter", "definition", e.def.ID, "filter", f.Name)
	return data, nil
}

func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// encode returns s in an encoding; UTF-8 when nil.
func encode(s string, enc encoding.Encoding) []byte {
	if enc == nil {
		return []byte(s)
	}
	b, err := enc.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return []byte(s)
	}
	return b
}

func decode(b []byte, enc encoding.Encoding) string {
	if enc == nil {
		return string(b)
	}
	s, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(s)
}

// urlEncode encodes s, in an encoding, as .NET's WebUtility.UrlEncode
// does: spaces as +, and all but letters, digits and -_.!*() escaped.
func urlEncode(s string, enc encoding.Encoding) string {
	var b strings.Builder
	for _, c := range encode(s, enc) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte("-_.!*()", c) >= 0:
			b.WriteByte(c)
		case c == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// urlDecode decodes s as .NET's WebUtility.UrlDecode does, in an
// encoding.
func urlDecode(s string, enc encoding.Encoding) string {
	in := encode(s, enc)
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		switch c := in[i]; {
		case c == '+':
			out = append(out, ' ')
		case c == '%' && i+2 < len(in) && isHex(in[i+1]) && isHex(in[i+2]):
			n, _ := strconv.ParseUint(string(in[i+1:i+3]), 16, 8)
			out = append(out, byte(n))
			i += 2
		default:
			out = append(out, c)
		}
	}
	return decode(out, enc)
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// htmlEncode encodes s as .NET's WebUtility.HtmlEncode does.
func htmlEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\'':
			b.WriteString("&#39;")
		case r == '&':
			b.WriteString("&amp;")
		case r >= 160 && r < 256, r > 0xFFFF:
			fmt.Fprintf(&b, "&#%d;", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validFileName replaces the characters file names may not have with _.
func validFileName(s string) string {
	out := strings.Map(func(r rune) rune {
		if r == 0 || r == '/' {
			return '_'
		}
		return r
	}, s)
	if out == "" {
		return "_"
	}
	return out
}

// removeDiacritics replaces letters with diacritics by their base letters.
func removeDiacritics(s string) string {
	d := norm.NFD.String(s)
	d = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, d)
	return norm.NFC.String(d)
}

// queryString joins query arguments as Prowlarr's GetQueryString does:
// keys as given, values encoded.
func queryString(args []Entry[string], enc encoding.Encoding, sep string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, a.Key+"="+urlEncode(a.Value, enc))
	}
	return strings.Join(parts, sep)
}

// nameValues are query arguments as .NET's NameValueCollection keeps
// them: repeated keys' values joined by commas.
type nameValues []Entry[string]

func (n *nameValues) add(key, value string) {
	for i := range *n {
		if (*n)[i].Key == key {
			(*n)[i].Value += "," + value
			return
		}
	}
	*n = append(*n, Entry[string]{key, value})
}

func (n *nameValues) set(key, value string) {
	for i := range *n {
		if (*n)[i].Key == key {
			(*n)[i].Value = value
			return
		}
	}
	*n = append(*n, Entry[string]{key, value})
}
