package main

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyWork = "openlibrary"
	keyISBN = "isbn"
)

// maxSubjects bounds the subjects taken as genres.
const maxSubjects = 10

// isBook reports whether Open Library describes items of a kind.
func isBook(k pluginv1.MediaKind) bool {
	return k == pluginv1.MediaKind_MEDIA_KIND_BOOK || k == pluginv1.MediaKind_MEDIA_KIND_AUDIOBOOK
}

func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	l := req.GetLookup()
	if !isBook(l.GetKind()) {
		return &pluginv1.SearchResponse{}, nil
	}
	c := p.client()
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 10
	}
	if id, err := p.workOf(ctx, c, l); err != nil || id != "" {
		if err != nil {
			return nil, err
		}
		w, err := c.work(ctx, id)
		if errors.Is(err, errNotFound) {
			return &pluginv1.SearchResponse{}, nil
		} else if err != nil {
			return nil, err
		}
		r := pluginv1.SearchResult_builder{
			Name: proto.String(w.Title), Overview: proto.String(string(w.Description)),
			ExternalIds: map[string]string{keyWork: id}, Score: proto.Float64(1),
		}.Build()
		if y := year(w.FirstPublishDate); y > 0 {
			r.SetYear(y)
		}
		if len(w.Covers) > 0 {
			r.SetImageUrl(c.coverURL(w.Covers[0]))
		}
		return pluginv1.SearchResponse_builder{Results: []*pluginv1.SearchResult{r}}.Build(), nil
	}
	if l.GetName() == "" {
		return &pluginv1.SearchResponse{}, nil
	}
	docs, err := c.search(ctx, l.GetName(), first(l.GetArtists()), limit)
	if err != nil {
		return nil, err
	}
	var results []*pluginv1.SearchResult
	for i, d := range docs {
		name := d.Title
		if len(d.AuthorNames) > 0 {
			name = strings.Join(d.AuthorNames, ", ") + " – " + d.Title
		}
		r := pluginv1.SearchResult_builder{
			Name: proto.String(name), Year: proto.Int32(int32(d.FirstPublishYear)),
			ExternalIds: map[string]string{keyWork: workID(d.Key)},
			// Open Library ranks by relevance without a score.
			Score: proto.Float64(1 / float64(i+1)),
		}.Build()
		if d.CoverID > 0 {
			r.SetImageUrl(c.coverURL(d.CoverID))
		}
		results = append(results, r)
	}
	return pluginv1.SearchResponse_builder{Results: results}.Build(), nil
}

// workOf returns the work an item's IDs name: its work ID, or the work of
// the edition its ISBN names; empty when it has neither.
func (p *plugin) workOf(ctx context.Context, c *client, l *pluginv1.Lookup) (string, error) {
	ids := l.GetExternalIds()
	if id := ids[keyWork]; id != "" {
		return id, nil
	}
	isbn := normalizeISBN(ids[keyISBN])
	if isbn == "" {
		return "", nil
	}
	e, err := c.edition(ctx, isbn)
	if errors.Is(err, errNotFound) || err == nil && len(e.Works) == 0 {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return workID(e.Works[0].Key), nil
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	l := req.GetLookup()
	notFound := pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build()
	if !isBook(l.GetKind()) {
		return notFound, nil
	}
	c := p.client()
	id, err := p.workOf(ctx, c, l)
	if err != nil {
		return nil, err
	}
	var authors []string
	if id == "" && l.GetName() != "" {
		docs, err := c.search(ctx, l.GetName(), first(l.GetArtists()), 1)
		if err != nil {
			return nil, err
		}
		if len(docs) > 0 {
			id, authors = workID(docs[0].Key), docs[0].AuthorNames
		}
	}
	if id == "" {
		return notFound, nil
	}
	w, err := c.work(ctx, id)
	if errors.Is(err, errNotFound) {
		return notFound, nil
	} else if err != nil {
		return nil, err
	}
	if len(authors) == 0 {
		for _, a := range w.Authors {
			if a.Author.Key == "" || len(authors) == 3 {
				continue
			}
			name, err := c.authorName(ctx, a.Author.Key)
			if err != nil && !errors.Is(err, errNotFound) {
				return nil, err
			}
			if name != "" {
				authors = append(authors, name)
			}
		}
	}
	ids := map[string]string{keyWork: id}
	if isbn := normalizeISBN(l.GetExternalIds()[keyISBN]); isbn != "" {
		ids[keyISBN] = isbn
	}
	name := w.Title
	if w.Subtitle != "" {
		name += ": " + w.Subtitle
	}
	md := pluginv1.Metadata_builder{
		Name:        proto.String(name),
		Overview:    proto.String(cleanDescription(string(w.Description))),
		Genres:      w.Subjects[:min(len(w.Subjects), maxSubjects)],
		ExternalIds: ids,
	}.Build()
	if d, ok := isoDate(w.FirstPublishDate); ok {
		md.SetPremiereDate(d)
	}
	if y := year(w.FirstPublishDate); y > 0 {
		md.SetProductionYear(y)
	}
	for i, a := range authors {
		md.SetPeople(append(md.GetPeople(), pluginv1.PersonCredit_builder{
			Name: proto.String(a), Kind: pluginv1.CreditKind_CREDIT_KIND_AUTHOR.Enum(), Order: proto.Int32(int32(i)),
		}.Build()))
	}
	for _, cover := range w.Covers {
		if cover > 0 {
			md.SetImages(append(md.GetImages(), pluginv1.RemoteImage_builder{
				Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String(c.coverURL(cover)),
			}.Build()))
		}
	}
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

// normalizeISBN keeps the digits and check character of an ISBN; empty
// when it is not one.
func normalizeISBN(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r == 'X' {
			b.WriteRune(r)
		}
	}
	if n := b.Len(); n != 10 && n != 13 {
		return ""
	}
	return b.String()
}

// sourceNote is the trailing source note and links Open Library
// descriptions often carry.
var sourceNote = regexp.MustCompile(`(?s)\s*(\(\[source\]\[\d+\]\)|-{5,}|\[\d+\]: https?://).*$`)

// cleanDescription drops the source notes from a description.
func cleanDescription(s string) string {
	return strings.TrimSpace(sourceNote.ReplaceAllString(s, ""))
}

// year finds the year of a free-form date such as "1954" or
// "July 29, 1954".
func year(s string) int32 {
	for f := range strings.FieldsFuncSeq(s, func(r rune) bool { return r < '0' || r > '9' }) {
		if len(f) == 4 {
			if y, err := strconv.Atoi(f); err == nil && y > 0 {
				return int32(y)
			}
		}
	}
	return 0
}

// isoDate parses the full dates among Open Library's free-form ones.
func isoDate(s string) (string, bool) {
	for _, layout := range []string{"January 2, 2006", "Jan 2, 2006", "2 January 2006", time.DateOnly} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t.Format(time.DateOnly), true
		}
	}
	return "", false
}

func first(s []string) string {
	if i := slices.IndexFunc(s, func(v string) bool { return v != "" }); i >= 0 {
		return s[i]
	}
	return ""
}
