package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// MaxGeneratedImageSize bounds the images generators make.
const MaxGeneratedImageSize = 8 << 20

// ImageGenerator makes images of items that have none, such as an image
// generator plugin.
type ImageGenerator interface {
	Name() string
	// GenerateImages returns images of some of the kinds asked for, at
	// most one of each.
	GenerateImages(ctx context.Context, it core.Item, kinds []core.ImageKind) ([]GeneratedImage, error)
}

// GeneratedImage is an image a generator made.
type GeneratedImage struct {
	Kind core.ImageKind
	// Content is a JPEG, PNG or WebP image.
	Content []byte
}

func (r *Refresher) generators() []ImageGenerator {
	if r.Generators == nil {
		return nil
	}
	return r.Generators()
}

// generatedFolder is the folder within MetadataDir keeping the images
// made for an item.
func (r *Refresher) generatedFolder(id core.ID) string {
	return path.Join(r.storedFolder(id), "generated")
}

// generate returns images of the kinds an item lacks, given the images it
// has: those made before, kept in MetadataDir, and else what the
// generators make, in their order. renew makes them anew. Images made for
// kinds the item now has otherwise are removed.
func (r *Refresher) generate(ctx context.Context, it core.Item, have []core.Image, renew bool) []core.Image {
	if r.MetadataDir == "" {
		return nil
	}
	dir := r.generatedFolder(it.ID)
	prefix := filepath.ToSlash(filepath.Join(r.MetadataDir, filepath.FromSlash(dir))) + "/"
	has := func(kind core.ImageKind) bool {
		return slices.ContainsFunc(have, func(img core.Image) bool { return img.Kind == kind && !strings.HasPrefix(img.Path, prefix) })
	}
	rt, err := os.OpenRoot(r.MetadataDir)
	if err != nil {
		r.logger().WarnContext(ctx, "generating images failed", "item", it.ID, "err", err)
		return nil
	}
	defer rt.Close()
	made := map[core.ImageKind]string{}
	if entries, err := fs.ReadDir(rt.FS(), dir); err == nil {
		for _, e := range entries {
			kind := core.ImageKind(strings.TrimSuffix(e.Name(), path.Ext(e.Name())))
			file := path.Join(dir, e.Name())
			if e.IsDir() || !kind.Valid() || renew || has(kind) {
				if err := rt.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
					r.logger().WarnContext(ctx, "removing a generated image failed", "item", it.ID, "file", file, "err", err)
				}
				continue
			}
			made[kind] = file
		}
	}
	var ask []core.ImageKind
	for _, kind := range core.ImageKinds {
		if _, ok := made[kind]; !ok && !has(kind) {
			ask = append(ask, kind)
		}
	}
	for _, g := range r.generators() {
		if len(ask) == 0 {
			break
		}
		images, err := g.GenerateImages(ctx, it, slices.Clone(ask))
		if err != nil {
			r.logger().WarnContext(ctx, "image generator failed", "generator", g.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, img := range images {
			if err := r.keepGenerated(rt, dir, ask, img, made); err != nil {
				r.logger().WarnContext(ctx, "image generator failed", "generator", g.Name(), "item", it.ID, "kind", img.Kind, "err", err)
				continue
			}
			ask = slices.DeleteFunc(ask, func(k core.ImageKind) bool { return k == img.Kind })
		}
	}
	var out []core.Image
	for _, kind := range core.ImageKinds {
		if file, ok := made[kind]; ok {
			out = append(out, core.Image{OwnerID: it.ID, Kind: kind, Path: filepath.ToSlash(filepath.Join(r.MetadataDir, filepath.FromSlash(file)))})
		}
	}
	return out
}

// keepGenerated checks an image a generator made of one of the kinds
// asked for and writes it to dir.
func (r *Refresher) keepGenerated(rt *os.Root, dir string, ask []core.ImageKind, img GeneratedImage, made map[core.ImageKind]string) error {
	switch {
	case !slices.Contains(ask, img.Kind):
		return fmt.Errorf("an image of kind %q was not asked for", img.Kind)
	case len(img.Content) > MaxGeneratedImageSize:
		return fmt.Errorf("%d bytes; at most %d", len(img.Content), MaxGeneratedImageSize)
	}
	file, err := writeArt(rt, path.Join(dir, string(img.Kind)), img.Content)
	if err != nil {
		return err
	}
	made[img.Kind] = file
	return nil
}

// withoutGenerated returns an item's images apart from those made for it.
func (r *Refresher) withoutGenerated(images []core.Image, id core.ID) []core.Image {
	if r.MetadataDir == "" {
		return images
	}
	prefix := filepath.ToSlash(filepath.Join(r.MetadataDir, filepath.FromSlash(r.generatedFolder(id)))) + "/"
	return slices.DeleteFunc(slices.Clone(images), func(img core.Image) bool { return strings.HasPrefix(img.Path, prefix) })
}
