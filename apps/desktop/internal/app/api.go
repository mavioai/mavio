package app

import (
	"fmt"
	"strings"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// itemService returns the ItemService client of the server with id.
func (a *App) itemService(id string) libraryv1connect.ItemServiceClient {
	return libraryv1connect.NewItemServiceClient(a.connectHTTP(), serverURL(id))
}

// imageOf returns an item's or person's first image of a kind that has a
// URL, as the core's artwork, nil without one.
func (a *App) imageOf(server string, images []*libraryv1.Image, kind libraryv1.ImageKind) *capiv1.Artwork {
	var best *libraryv1.Image
	for _, i := range images {
		if i.GetKind() == kind && i.GetUrl() != "" && (best == nil || i.GetIndex() < best.GetIndex()) {
			best = i
		}
	}
	if best == nil {
		return nil
	}
	b := capiv1.Artwork_builder{Url: a.serverURLOf(server, best.GetUrl()), Blurhash: best.GetBlurhash()}
	if best.GetHeight() > 0 {
		b.Aspect = float32(best.GetWidth()) / float32(best.GetHeight())
	}
	return b.Build()
}

// cardOf returns the card of an item of a server as the core makes it.
func (a *App) cardOf(server string, it *libraryv1.Item) *capiv1.Card {
	kind := capiv1.CardKind(it.GetKind())
	b := capiv1.Card_builder{
		ServerId: server, ItemId: it.GetId(), Kind: kind, Title: it.GetName(), Year: it.GetProductionYear(),
		Poster: a.imageOf(server, it.GetImages(), libraryv1.ImageKind_IMAGE_KIND_PRIMARY),
		Thumb:  a.imageOf(server, it.GetImages(), libraryv1.ImageKind_IMAGE_KIND_THUMB),
	}
	if b.Thumb == nil {
		b.Thumb = a.imageOf(server, it.GetImages(), libraryv1.ImageKind_IMAGE_KIND_BACKDROP)
	}
	if it.HasRuntime() {
		b.RuntimeMs = it.GetRuntime().AsDuration().Milliseconds()
	}
	if u := it.GetUserData(); u != nil {
		b.Played, b.Favorite = u.GetPlayed(), u.GetFavorite()
		if u.HasPosition() && b.RuntimeMs > 0 {
			b.Progress = new(float32(u.GetPosition().AsDuration().Milliseconds()) / float32(b.RuntimeMs))
		}
	}
	switch kind {
	case capiv1.CardKind_CARD_KIND_EPISODE:
		var parts []string
		if it.GetSeriesName() != "" {
			parts = append(parts, it.GetSeriesName())
		}
		switch {
		case it.GetParentIndexNumber() > 0 && it.GetIndexNumber() > 0:
			parts = append(parts, fmt.Sprintf("S%d:E%d", it.GetParentIndexNumber(), it.GetIndexNumber()))
		case it.GetIndexNumber() > 0:
			parts = append(parts, fmt.Sprintf("E%d", it.GetIndexNumber()))
		}
		b.Subtitle = strings.Join(parts, " · ")
	case capiv1.CardKind_CARD_KIND_SEASON:
		b.Subtitle = it.GetSeriesName()
	case capiv1.CardKind_CARD_KIND_ALBUM, capiv1.CardKind_CARD_KIND_TRACK:
		artists := it.GetAlbumArtists()
		if len(artists) == 0 {
			artists = it.GetArtists()
		}
		b.Subtitle = strings.Join(artists, ", ")
	}
	return b.Build()
}
