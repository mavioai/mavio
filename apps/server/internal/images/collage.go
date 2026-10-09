package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
)

// collageCandidates bounds the items read to find a collage's images.
const collageCandidates = 50

// latestTopKinds are the items a library's collage shows.
var latestTopKinds = []core.ItemKind{
	core.KindMovie, core.KindSeries, core.KindMusicAlbum, core.KindMusicVideo, core.KindVideo,
	core.KindBook, core.KindAudioBook, core.KindPhotoAlbum,
}

// serveCollage serves GET /images/collages/{id}: the collage of a library,
// collection or playlist, with the size parameters of other images. Like
// an image ID, the owner's ID is the credential.
func (s *Server) serveCollage(w http.ResponseWriter, r *http.Request) {
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
	if opts.Format == "" {
		w.Header().Set("Vary", "Accept")
		if strings.Contains(r.Header.Get("Accept"), "image/webp") {
			opts.Format = imaging.WebP
		}
	}
	images, width, height, err := s.collageImages(r.Context(), id)
	if errors.Is(err, core.ErrNotFound) || err == nil && len(images) == 0 {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.fail(w, r, id, err)
		return
	}

	// The collage changes with the images it shows.
	h := sha256.New()
	h.Write(id[:])
	for _, img := range images {
		h.Write(img.ID[:])
	}
	key := hex.EncodeToString(h.Sum(nil)[:16]) + "-" + optionsKey(opts)
	data, contentType, err := s.renderCollage(r.Context(), key, images, width, height, opts)
	if errors.Is(err, imaging.ErrNoImages) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		s.fail(w, r, id, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", `"`+key+`"`)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// collageImages chooses a collage's images and size: for a library the
// primary images of its latest items, side by side; for a collection its
// items' posters; for a playlist the posters of its items' series and
// albums, or its items'.
func (s *Server) collageImages(ctx context.Context, id core.ID) ([]core.Image, int, int, error) {
	var owners []core.ID
	width, height := 600, 900
	lib, err := s.cfg.Store.Libraries().Get(ctx, id)
	switch {
	case err == nil && !lib.Kind.Curated():
		width, height = 960, 540
		page, err := s.cfg.Store.Items().Query(ctx, core.ItemQuery{
			LibraryIDs: []core.ID{lib.ID}, Kinds: latestTopKinds, Limit: collageCandidates,
			Sort: []core.SortSpec{{Field: core.SortDateAdded, Desc: true}},
		})
		if err != nil {
			return nil, 0, 0, err
		}
		for _, it := range page.Items {
			owners = append(owners, it.ID)
		}
	case err == nil:
		return nil, 0, 0, core.ErrNotFound
	case !errors.Is(err, core.ErrNotFound):
		return nil, 0, 0, err
	default:
		it, err := s.cfg.Store.Items().Get(ctx, id)
		if err != nil {
			return nil, 0, 0, err
		}
		if !it.Kind.IsCurated() {
			return nil, 0, 0, core.ErrNotFound
		}
		links, err := s.cfg.Store.Items().Links(ctx, it.ID)
		if err != nil {
			return nil, 0, 0, err
		}
		if it.Kind == core.KindPlaylist {
			height = 600
		}
		seen := map[core.ID]bool{}
		for _, l := range links[:min(len(links), collageCandidates)] {
			owner := l.ItemID
			if it.Kind == core.KindPlaylist {
				if owner, err = s.posterOwner(ctx, owner); err != nil {
					return nil, 0, 0, err
				}
			}
			if !seen[owner] {
				seen[owner] = true
				owners = append(owners, owner)
			}
		}
	}

	byOwner, err := s.cfg.Store.Images().ListForOwners(ctx, owners)
	if err != nil {
		return nil, 0, 0, err
	}
	var out []core.Image
	for _, owner := range owners {
		for _, img := range byOwner[owner] {
			if img.Kind == core.ImagePrimary && img.Index == 0 {
				out = append(out, img)
				break
			}
		}
	}
	return out, width, height, nil
}

// posterOwner returns whose poster stands for a playlist item: an
// episode's series, a track's album, or the item itself.
func (s *Server) posterOwner(ctx context.Context, id core.ID) (core.ID, error) {
	it, err := s.cfg.Store.Items().Get(ctx, id)
	if err != nil {
		return id, err
	}
	want := map[core.ItemKind]core.ItemKind{core.KindEpisode: core.KindSeries, core.KindTrack: core.KindMusicAlbum}[it.Kind]
	for parent := it.ParentID; want != "" && !parent.IsZero(); {
		p, err := s.cfg.Store.Items().Get(ctx, parent)
		if errors.Is(err, core.ErrNotFound) {
			break
		} else if err != nil {
			return id, err
		}
		if p.Kind == want {
			return p.ID, nil
		}
		parent = p.ParentID
	}
	return id, nil
}

// renderCollage returns a collage rendered with opts, from the cache when
// rendered before. Images that cannot be read are left out.
func (s *Server) renderCollage(ctx context.Context, key string, images []core.Image, width, height int, opts imaging.Options) ([]byte, string, error) {
	dir := filepath.Join(s.cfg.Dir, "collages")
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
	v, err, _ := s.group.Do("collage:"+key, func() (any, error) {
		var sources [][]byte
		for i := range images {
			if len(sources) == imaging.CollageImages {
				break
			}
			data, err := s.original(ctx, &images[i])
			if err != nil {
				s.log.WarnContext(ctx, "collage image", "image", images[i].ID, "err", err)
				continue
			}
			if !isSVG(&images[i], data) {
				sources = append(sources, data)
			}
		}
		collage, err := imaging.Collage(sources, width, height)
		if err != nil {
			return nil, err
		}
		data, format, err := imaging.Render(collage, opts)
		if err != nil {
			return nil, err
		}
		ext, _ := format.Extension()
		if err := writeFile(filepath.Join(dir, key+ext), data); err != nil {
			s.log.WarnContext(ctx, "cache collage", "err", err)
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
