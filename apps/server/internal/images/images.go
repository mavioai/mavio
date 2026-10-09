// Package images serves artwork over HTTP: local files from the library
// folders and provider images downloaded once, rendered at the requested
// size and cached.
package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
	"github.com/mavioai/mavio/libs/library"
)

// maxDownload bounds a downloaded provider image.
const maxDownload = 50 << 20

// errUnavailable is returned for images the server cannot read.
var errUnavailable = errors.New("image unavailable")

// Config configures a Server.
type Config struct {
	Store core.Store
	// Dir caches downloaded and rendered images.
	Dir string
	// MetadataDir holds the images chosen for items, beside those in the
	// library folders.
	MetadataDir string
	// Client downloads provider images; nil uses one with a 30 second
	// timeout.
	Client *http.Client
	Logger *slog.Logger
}

// Server serves images by ID at /images/{id}. The ID, which clients only
// learn from API responses filtered by their access, is the only
// credential.
type Server struct {
	cfg   Config
	log   *slog.Logger
	group singleflight.Group
}

// New returns an image server.
func New(cfg Config) *Server {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Server{cfg: cfg, log: log}
}

// Handler serves GET /images/{id}, optionally rendered with the query
// parameters width, height, maxWidth, maxHeight, fillWidth, fillHeight
// and quality, as Jellyfin's image API takes them, and format. Renderings
// without a format are WebP for clients that accept it, else JPEG or PNG.
// It also serves GET /images/collages/{id}, the collage of a library,
// collection or playlist, with the same parameters.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /images/{id}", s.serve)
	mux.HandleFunc("GET /images/collages/{id}", s.serveCollage)
	return mux
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	id, err := core.ParseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	opts, err := parseOptions(r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resize := opts != (imaging.Options{})
	if opts.Format == "" {
		// Renderings are WebP for clients that take it.
		w.Header().Set("Vary", "Accept")
		if strings.Contains(r.Header.Get("Accept"), "image/webp") {
			opts.Format = imaging.WebP
		}
	}
	img, err := s.cfg.Store.Images().Get(r.Context(), id)
	if errors.Is(err, core.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.fail(w, r, id, err)
		return
	}
	original, err := s.original(r.Context(), &img)
	if errors.Is(err, errUnavailable) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.fail(w, r, id, err)
		return
	}

	sum := sha256.Sum256(original)
	etag := hex.EncodeToString(sum[:16])
	var data []byte
	var contentType string
	switch {
	case isSVG(&img, original):
		// SVGs are served as they are once checked to reference nothing
		// outside themselves.
		if err := imaging.CheckSVG(bytes.NewReader(original)); err != nil {
			s.log.WarnContext(r.Context(), "unsafe SVG", "image", id, "err", err)
			http.NotFound(w, r)
			return
		}
		data, contentType = original, "image/svg+xml"
	case !resize:
		data, contentType = original, http.DetectContentType(original)
	default:
		key := etag + "-" + optionsKey(opts)
		etag = key
		data, contentType, err = s.render(r.Context(), key, original, opts)
		if err != nil {
			s.fail(w, r, id, err)
			return
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, id core.ID, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	s.log.ErrorContext(r.Context(), "serve image", "image", id, "err", err)
	http.Error(w, "image unavailable", http.StatusInternalServerError)
}

// original returns the image's bytes: a local file within its item's
// library, or a provider image downloaded once.
func (s *Server) original(ctx context.Context, img *core.Image) ([]byte, error) {
	if img.Path != "" {
		return s.local(ctx, img)
	}
	if img.RemoteURL == "" {
		return nil, errUnavailable
	}
	file := filepath.Join(s.cfg.Dir, "originals", img.ID.String())
	if data, err := os.ReadFile(file); err == nil {
		return data, nil
	}
	v, err, _ := s.group.Do("download:"+img.ID.String(), func() (any, error) {
		// Downloads finish for later requests even if this one goes away.
		return s.download(context.WithoutCancel(ctx), img.RemoteURL, file)
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// Analyze measures an image and computes its placeholders, as a
// library.ImageAnalyzer; SVGs give zero facts.
func (s *Server) Analyze(ctx context.Context, img core.Image) (library.ImageFacts, error) {
	data, err := s.original(ctx, &img)
	if err != nil {
		return library.ImageFacts{}, err
	}
	if isSVG(&img, data) {
		return library.ImageFacts{}, nil
	}
	p, err := imaging.Placeholders(data)
	if err != nil {
		return library.ImageFacts{}, err
	}
	return library.ImageFacts{Width: p.Width, Height: p.Height, Blurhash: p.Blurhash, Thumbhash: p.Thumbhash}, nil
}

func (s *Server) local(ctx context.Context, img *core.Image) ([]byte, error) {
	item, err := s.cfg.Store.Items().Get(ctx, img.OwnerID)
	if errors.Is(err, core.ErrNotFound) {
		// Local images of people are not kept yet.
		return nil, errUnavailable
	} else if err != nil {
		return nil, err
	}
	lib, err := s.cfg.Store.Libraries().Get(ctx, item.LibraryID)
	if err != nil {
		return nil, err
	}
	roots := lib.Paths
	if s.cfg.MetadataDir != "" {
		roots = append(slices.Clip(roots), s.cfg.MetadataDir)
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, filepath.FromSlash(img.Path))
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		f, err := os.OpenInRoot(root, rel)
		if err != nil {
			s.log.WarnContext(ctx, "open image", "path", img.Path, "err", err)
			return nil, errUnavailable
		}
		defer func() { _ = f.Close() }()
		return io.ReadAll(io.LimitReader(f, maxDownload))
	}
	return nil, errUnavailable
}

// download fetches a provider image into file.
func (s *Server) download(ctx context.Context, rawURL, file string) ([]byte, error) {
	data, err := s.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if err := writeFile(file, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Fetch downloads an image of at most 50 MiB over HTTP or HTTPS.
func (s *Server) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	if u, err := url.Parse(rawURL); err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("download image: %q is not an http or https URL: %w", rawURL, errUnavailable)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download image: %s: %w", resp.Status, errUnavailable)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("download image: larger than %d bytes: %w", maxDownload, errUnavailable)
	}
	return data, nil
}

// render returns the image rendered with opts, from the cache when
// rendered before.
func (s *Server) render(ctx context.Context, key string, original []byte, opts imaging.Options) ([]byte, string, error) {
	dir := filepath.Join(s.cfg.Dir, "rendered")
	for _, f := range []imaging.Format{imaging.JPEG, imaging.PNG, imaging.WebP} {
		ext, _ := f.Extension()
		if data, err := os.ReadFile(filepath.Join(dir, key+ext)); err == nil {
			mime, _ := f.MimeType()
			return data, mime, nil
		}
	}
	type rendered struct {
		data   []byte
		format imaging.Format
	}
	v, err, _ := s.group.Do("render:"+key, func() (any, error) {
		data, format, err := imaging.Process(original, opts)
		if err != nil {
			return nil, err
		}
		ext, _ := format.Extension()
		if err := writeFile(filepath.Join(dir, key+ext), data); err != nil {
			s.log.WarnContext(ctx, "cache rendered image", "err", err)
		}
		return rendered{data, format}, nil
	})
	if err != nil {
		return nil, "", err
	}
	r := v.(rendered)
	mime, _ := r.format.MimeType()
	return r.data, mime, nil
}

// writeFile writes data to file atomically.
func writeFile(file string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), file)
}

func isSVG(img *core.Image, data []byte) bool {
	if strings.EqualFold(filepath.Ext(img.Path), ".svg") {
		return true
	}
	head := bytes.TrimSpace(data[:min(len(data), 512)])
	return bytes.HasPrefix(head, []byte("<svg")) || (bytes.HasPrefix(head, []byte("<?xml")) && bytes.Contains(head, []byte("<svg")))
}

// parseOptions reads the size parameters, each 1–10000, the quality and
// the format: jpg, png or webp.
func parseOptions(q url.Values) (imaging.Options, error) {
	var o imaging.Options
	switch f := strings.ToLower(q.Get("format")); f {
	case "":
	case "jpg", "jpeg":
		o.Format = imaging.JPEG
	case "png", "webp":
		o.Format = imaging.Format(f)
	default:
		return o, fmt.Errorf("format must be jpg, png or webp")
	}
	for name, dst := range map[string]*int{
		"width": &o.Width, "height": &o.Height, "maxWidth": &o.MaxWidth, "maxHeight": &o.MaxHeight,
		"fillWidth": &o.FillWidth, "fillHeight": &o.FillHeight, "quality": &o.Quality,
	} {
		v := q.Get(name)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		limit := 10000
		if name == "quality" {
			limit = 100
		}
		if err != nil || n < 1 || n > limit {
			return o, fmt.Errorf("%s must be 1–%d", name, limit)
		}
		*dst = n
	}
	return o, nil
}

// optionsKey names a rendering in the cache.
func optionsKey(o imaging.Options) string {
	return fmt.Sprintf("w%d-h%d-mw%d-mh%d-fw%d-fh%d-q%d-%s", o.Width, o.Height, o.MaxWidth, o.MaxHeight, o.FillWidth, o.FillHeight, o.Quality, o.Format)
}
