package probe

import (
	"encoding/xml"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// itunesInfo reads the plist in an iTunMOVI tag: studios, screenwriters,
// producers and directors. Unlike Jellyfin, which keeps only the people of
// the last key, people of all keys are kept.
func itunesInfo(plist string, r *Result) {
	if i := strings.Index(strings.ToLower(plist), "<plist"); i >= 0 {
		plist = plist[i:]
	}
	d := xml.NewDecoder(strings.NewReader(plist))
	d.Strict = false
	// Find the top-level <dict>.
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "dict" && depth == 2 {
				readDict(d, r)
				return
			}
		case xml.EndElement:
			depth--
		}
	}
}

// readDict reads key/value pairs until the dict ends.
func readDict(d *xml.Decoder, r *Result) {
	key := ""
	for {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "dict" {
				return
			}
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				key = text(d)
			case "string":
				if v := strings.TrimSpace(text(d)); v != "" {
					pairs(key, []string{v}, r)
				}
			case "array":
				pairs(key, arrayNames(d), r)
			default:
				_ = d.Skip()
			}
		}
	}
}

// arrayNames reads an array of dicts with a "name" entry.
func arrayNames(d *xml.Decoder) []string {
	var names []string
	for {
		tok, err := d.Token()
		if err != nil {
			return names
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "array" {
				return names
			}
		case xml.StartElement:
			if t.Name.Local != "dict" {
				_ = d.Skip()
				continue
			}
			name, value := "", ""
			for {
				tok, err := d.Token()
				if err != nil {
					return names
				}
				if e, ok := tok.(xml.EndElement); ok && e.Name.Local == "dict" {
					break
				}
				if s, ok := tok.(xml.StartElement); ok {
					switch s.Name.Local {
					case "key":
						name = strings.TrimSpace(text(d))
					case "string":
						value = strings.TrimSpace(text(d))
					default:
						_ = d.Skip()
					}
				}
			}
			if name != "" && value != "" {
				names = append(names, value)
			}
		}
	}
}

func text(d *xml.Decoder) string {
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return b.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return b.String()
		case xml.StartElement:
			_ = d.Skip()
		}
	}
}

func pairs(key string, values []string, r *Result) {
	values = distinctFold(values)
	kind := core.CreditKind("")
	switch strings.ToLower(key) {
	case "studio":
		r.Metadata.Studios = values
		return
	case "screenwriters":
		kind = core.CreditWriter
	case "producers":
		kind = core.CreditProducer
	case "directors":
		kind = core.CreditDirector
	default:
		return
	}
	for _, v := range values {
		r.People = append(r.People, Person{Name: v, Kind: kind})
	}
}
