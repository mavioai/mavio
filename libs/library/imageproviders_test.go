package library

import (
	"context"
	"errors"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// fanart has posters and logos of every film it is asked about, by the
// TMDB ID the metadata providers found.
type fanart struct{ fail bool }

func (fanart) Name() string { return "fanart" }

func (f fanart) Images(_ context.Context, l Lookup) ([]metadata.RemoteImage, error) {
	if f.fail {
		return nil, errors.New("down")
	}
	base := "https://fanart.example/" + l.ExternalIDs[core.ProviderTMDB]
	return []metadata.RemoteImage{
		{Kind: core.ImagePrimary, URL: base + "/poster.jpg", Score: 1},
		{Kind: core.ImageLogo, URL: base + "/logo.png", Score: 0.5},
		{Kind: core.ImageLogo, URL: base + "/logo-hd.png", Score: 0.9},
	}, nil
}

func TestImageProviders(t *testing.T) {
	f := newManage(t, false)
	f.r.Images = func() []ImageProvider { return []ImageProvider{fanart{fail: true}, fanart{}} }
	it := f.refresh(RefreshOptions{})
	// The metadata provider's poster wins; the image provider adds the
	// best logo, found by the ID the metadata provider gave.
	got := f.images(it.ID)
	if got[core.ImagePrimary] != "https://image.example/2/poster.jpg" || got[core.ImageLogo] != "https://fanart.example/2/logo-hd.png" {
		t.Errorf("images = %v", got)
	}
	remote, err := f.r.RemoteImages(t.Context(), f.lib, it.ID, core.ImageLogo)
	if err != nil || len(remote) != 2 || remote[0].Provider != "fanart" || remote[1].URL != "https://fanart.example/2/logo-hd.png" {
		t.Errorf("RemoteImages(logo) = %+v, %v", remote, err)
	}
}
