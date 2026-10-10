package library

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// painter makes logos, and banners that are not images, and a disc nobody
// asked for once the logo is there.
type painter struct {
	fail  bool
	asked [][]core.ImageKind
}

func (*painter) Name() string { return "painter" }

func (p *painter) GenerateImages(_ context.Context, _ core.Item, kinds []core.ImageKind) ([]GeneratedImage, error) {
	if p.fail {
		return nil, errors.New("out of paint")
	}
	p.asked = append(p.asked, kinds)
	out := []GeneratedImage{{Kind: core.ImageBanner, Content: []byte("not an image")}}
	if slices.Contains(kinds, core.ImageLogo) {
		out = append(out, GeneratedImage{Kind: core.ImageLogo, Content: pngData})
	} else {
		out = append(out, GeneratedImage{Kind: core.ImageLogo, Content: jpegData})
	}
	return out, nil
}

func TestImageGenerators(t *testing.T) {
	f := newManage(t, false)
	p := &painter{}
	f.r.Generators = func() []ImageGenerator { return []ImageGenerator{&painter{fail: true}, p} }

	it := f.refresh(RefreshOptions{})
	logo := f.images(it.ID)[core.ImageLogo]
	if !strings.HasPrefix(logo, f.r.MetadataDir+"/") || !strings.HasSuffix(logo, "/generated/logo.png") {
		t.Errorf("logo = %q", logo)
	}
	// Kinds the providers gave are not asked for; a banner that is no
	// image is dropped.
	if len(p.asked) != 1 || slices.Contains(p.asked[0], core.ImagePrimary) || slices.Contains(p.asked[0], core.ImageBackdrop) ||
		!slices.Contains(p.asked[0], core.ImageLogo) {
		t.Errorf("asked for %v", p.asked)
	}
	if _, ok := f.images(it.ID)[core.ImageBanner]; ok {
		t.Error("the banner that is not an image was kept")
	}

	// The logo made before is kept, not asked for again; the logo given
	// though not asked for is refused.
	it = f.refresh(RefreshOptions{})
	if len(p.asked) != 2 || slices.Contains(p.asked[1], core.ImageLogo) || f.images(it.ID)[core.ImageLogo] != logo {
		t.Errorf("asked for %v; logo = %q, want %q", p.asked, f.images(it.ID)[core.ImageLogo], logo)
	}
	if !exists(f.r.MetadataDir, strings.TrimPrefix(logo, f.r.MetadataDir+"/")) {
		t.Error("the generated logo is gone")
	}

	// Replacing metadata makes the images anew.
	it = f.refresh(RefreshOptions{ReplaceMetadata: true})
	if len(p.asked) != 3 || !slices.Contains(p.asked[2], core.ImageLogo) || f.images(it.ID)[core.ImageLogo] != logo {
		t.Errorf("asked for %v; logo = %q", p.asked, f.images(it.ID)[core.ImageLogo])
	}

	// A logo of its own replaces the generated one, which is removed.
	if _, err := f.r.SetImage(t.Context(), f.lib, it.ID, core.ImageLogo, jpegData); err != nil {
		t.Fatal(err)
	}
	it = f.refresh(RefreshOptions{})
	if got := f.images(it.ID)[core.ImageLogo]; got == logo || exists(f.r.MetadataDir, strings.TrimPrefix(logo, f.r.MetadataDir+"/")) {
		t.Errorf("logo = %q; generated logo kept", got)
	}
}
