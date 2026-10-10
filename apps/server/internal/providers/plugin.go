package providers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Plugin is a metadata provider plugin as a library.Provider.
type Plugin struct {
	// ID names the plugin in logs, such as "org.mavio.scraper-tmdb".
	ID     string
	Client pluginv1connect.MetadataProviderServiceClient
}

// Name returns the plugin's ID.
func (p *Plugin) Name() string { return p.ID }

// Metadata asks the plugin for an item's metadata. Items the plugin cannot
// identify by their external IDs are searched for first, and the best
// result is taken; seasons and episodes are identified through their
// series. It returns nil when the plugin knows nothing of the item.
func (p *Plugin) Metadata(ctx context.Context, l library.Lookup) (*metadata.Result, error) {
	kind, ok := mediaKinds[l.Kind]
	if !ok {
		return nil, nil
	}
	lookup := toLookup(kind, l)
	if len(lookup.GetExternalIds()) == 0 && kind != pluginv1.MediaKind_MEDIA_KIND_SEASON && kind != pluginv1.MediaKind_MEDIA_KIND_EPISODE {
		if l.Name == "" {
			return nil, nil
		}
		found, err := p.Client.Search(ctx, pluginv1.SearchRequest_builder{Lookup: lookup, Limit: proto.Int32(1)}.Build())
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		if len(found.GetResults()) == 0 {
			return nil, nil
		}
		lookup.SetExternalIds(found.GetResults()[0].GetExternalIds())
	}
	resp, err := p.Client.GetMetadata(ctx, pluginv1.GetMetadataRequest_builder{Lookup: lookup}.Build())
	if err != nil {
		return nil, fmt.Errorf("get metadata: %w", err)
	}
	if !resp.GetFound() {
		return nil, nil
	}
	return toResult(resp.GetMetadata()), nil
}

// Search asks the plugin what an item may be.
func (p *Plugin) Search(ctx context.Context, l library.Lookup, limit int) ([]library.SearchResult, error) {
	kind, ok := mediaKinds[l.Kind]
	if !ok || l.Name == "" && len(l.ExternalIDs) == 0 {
		return nil, nil
	}
	found, err := p.Client.Search(ctx, pluginv1.SearchRequest_builder{Lookup: toLookup(kind, l), Limit: proto.Int32(int32(limit))}.Build())
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	out := make([]library.SearchResult, 0, len(found.GetResults()))
	for _, r := range found.GetResults() {
		out = append(out, library.SearchResult{
			Provider: p.ID, Name: r.GetName(), Year: int(r.GetYear()), Overview: r.GetOverview(),
			ExternalIDs: externalIDs(r.GetExternalIds()), ImageURL: r.GetImageUrl(), Score: r.GetScore(),
		})
	}
	return out, nil
}

// ImagePlugin is an image provider plugin as a library.ImageProvider.
type ImagePlugin struct {
	ID     string
	Client pluginv1connect.ImageProviderServiceClient
}

// Name returns the plugin's ID.
func (p *ImagePlugin) Name() string { return p.ID }

// Images asks the plugin for images of an item.
func (p *ImagePlugin) Images(ctx context.Context, l library.Lookup) ([]metadata.RemoteImage, error) {
	kind, ok := mediaKinds[l.Kind]
	if !ok {
		return nil, nil
	}
	resp, err := p.Client.GetImages(ctx, pluginv1.GetImagesRequest_builder{Lookup: toLookup(kind, l)}.Build())
	if err != nil {
		return nil, fmt.Errorf("get images: %w", err)
	}
	return remoteImages(resp.GetImages()), nil
}

// externalIDs converts a contract's external IDs; nil when there are none.
func externalIDs(m map[string]string) map[core.Provider]string {
	var out map[core.Provider]string
	for k, v := range m {
		if v == "" {
			continue
		}
		if out == nil {
			out = map[core.Provider]string{}
		}
		out[core.Provider(k)] = v
	}
	return out
}

var mediaKinds = map[core.ItemKind]pluginv1.MediaKind{
	core.KindMovie:       pluginv1.MediaKind_MEDIA_KIND_MOVIE,
	core.KindSeries:      pluginv1.MediaKind_MEDIA_KIND_SERIES,
	core.KindSeason:      pluginv1.MediaKind_MEDIA_KIND_SEASON,
	core.KindEpisode:     pluginv1.MediaKind_MEDIA_KIND_EPISODE,
	core.KindMusicArtist: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST,
	core.KindMusicAlbum:  pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM,
	core.KindTrack:       pluginv1.MediaKind_MEDIA_KIND_TRACK,
	core.KindMusicVideo:  pluginv1.MediaKind_MEDIA_KIND_MUSIC_VIDEO,
	core.KindBook:        pluginv1.MediaKind_MEDIA_KIND_BOOK,
	core.KindAudioBook:   pluginv1.MediaKind_MEDIA_KIND_AUDIOBOOK,
}

// ItemKind returns the kind of items of a media kind; persons and unknown
// kinds have none.
func ItemKind(kind pluginv1.MediaKind) (core.ItemKind, bool) {
	for k, v := range mediaKinds {
		if v == kind {
			return k, true
		}
	}
	return "", false
}

func toLookup(kind pluginv1.MediaKind, l library.Lookup) *pluginv1.Lookup {
	lookup := pluginv1.Lookup_builder{
		Kind:              kind.Enum(),
		Name:              proto.String(l.Name),
		Year:              proto.Int32(int32(l.Year)),
		ExternalIds:       ids(l.ExternalIDs),
		SeriesExternalIds: ids(l.SeriesExternalIDs),
		Album:             proto.String(l.Album),
		Artists:           l.Artists,
		Language:          proto.String(l.Language),
		Country:           proto.String(l.Country),
	}.Build()
	if l.SeasonNumber != nil {
		lookup.SetSeasonNumber(int32(*l.SeasonNumber))
	}
	if l.EpisodeNumber != nil {
		lookup.SetEpisodeNumber(int32(*l.EpisodeNumber))
	}
	return lookup
}

func ids(m map[core.Provider]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if v != "" {
			out[string(k)] = v
		}
	}
	return out
}

func toResult(md *pluginv1.Metadata) *metadata.Result {
	it := core.Item{
		Name:           md.GetName(),
		OriginalTitle:  md.GetOriginalTitle(),
		SortName:       md.GetSortName(),
		Overview:       md.GetOverview(),
		Tagline:        md.GetTagline(),
		ProductionYear: int(md.GetProductionYear()),
		PremiereDate:   date(md.GetPremiereDate()),
		EndDate:        date(md.GetEndDate()),
		OfficialRating: md.GetOfficialRating(),
		Genres:         md.GetGenres(),
		Tags:           md.GetTags(),
		Studios:        md.GetStudios(),
		Artists:        md.GetArtists(),
		AlbumArtists:   md.GetAlbumArtists(),
		SeriesStatus:   seriesStatuses[md.GetSeriesStatus()],
		CollectionName: md.GetCollectionName(),
	}
	if md.HasRuntime() {
		it.Runtime = md.GetRuntime().AsDuration()
	}
	// Out-of-range ratings would make the item invalid.
	if r := md.GetCommunityRating(); r >= 0 && r <= 10 {
		it.CommunityRating = r
	}
	if r := md.GetCriticRating(); r >= 0 && r <= 100 {
		it.CriticRating = r
	}
	it.ExternalIDs = externalIDs(md.GetExternalIds())
	res := &metadata.Result{Item: it}
	for _, pc := range md.GetPeople() {
		if pc.GetName() == "" {
			continue
		}
		p := metadata.Person{Name: pc.GetName(), Kind: creditKinds[pc.GetKind()], Role: pc.GetRole(), ImageURL: pc.GetImageUrl()}
		if p.Kind == "" {
			p.Kind = core.CreditOther
		}
		if pc.HasOrder() {
			order := int(pc.GetOrder())
			p.Order = &order
		}
		res.People = append(res.People, p)
	}
	res.RemoteImages = remoteImages(md.GetImages())
	return res
}

// remoteImages converts a contract's images, leaving out those of unknown
// kinds or without URLs.
func remoteImages(images []*pluginv1.RemoteImage) []metadata.RemoteImage {
	var out []metadata.RemoteImage
	for _, img := range images {
		if kind, ok := imageKinds[img.GetKind()]; ok && img.GetUrl() != "" {
			out = append(out, metadata.RemoteImage{
				Kind: kind, URL: img.GetUrl(), Width: int(img.GetWidth()), Height: int(img.GetHeight()),
				Language: img.GetLanguage(), Score: img.GetScore(),
			})
		}
	}
	return out
}

// date parses an ISO 8601 calendar date; nil when empty or malformed.
func date(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &t
}

var seriesStatuses = map[pluginv1.SeriesStatus]core.SeriesStatus{
	pluginv1.SeriesStatus_SERIES_STATUS_CONTINUING: core.SeriesContinuing,
	pluginv1.SeriesStatus_SERIES_STATUS_ENDED:      core.SeriesEnded,
	pluginv1.SeriesStatus_SERIES_STATUS_UNRELEASED: core.SeriesUnreleased,
}

var creditKinds = map[pluginv1.CreditKind]core.CreditKind{
	pluginv1.CreditKind_CREDIT_KIND_ACTOR:      core.CreditActor,
	pluginv1.CreditKind_CREDIT_KIND_GUEST_STAR: core.CreditGuestStar,
	pluginv1.CreditKind_CREDIT_KIND_DIRECTOR:   core.CreditDirector,
	pluginv1.CreditKind_CREDIT_KIND_WRITER:     core.CreditWriter,
	pluginv1.CreditKind_CREDIT_KIND_PRODUCER:   core.CreditProducer,
	pluginv1.CreditKind_CREDIT_KIND_CREATOR:    core.CreditCreator,
	pluginv1.CreditKind_CREDIT_KIND_COMPOSER:   core.CreditComposer,
	pluginv1.CreditKind_CREDIT_KIND_CONDUCTOR:  core.CreditConductor,
	pluginv1.CreditKind_CREDIT_KIND_LYRICIST:   core.CreditLyricist,
	pluginv1.CreditKind_CREDIT_KIND_ARTIST:     core.CreditArtist,
	pluginv1.CreditKind_CREDIT_KIND_AUTHOR:     core.CreditAuthor,
	pluginv1.CreditKind_CREDIT_KIND_NARRATOR:   core.CreditNarrator,
	pluginv1.CreditKind_CREDIT_KIND_OTHER:      core.CreditOther,
}

var imageKinds = map[pluginv1.ImageKind]core.ImageKind{
	pluginv1.ImageKind_IMAGE_KIND_PRIMARY:    core.ImagePrimary,
	pluginv1.ImageKind_IMAGE_KIND_BACKDROP:   core.ImageBackdrop,
	pluginv1.ImageKind_IMAGE_KIND_LOGO:       core.ImageLogo,
	pluginv1.ImageKind_IMAGE_KIND_THUMB:      core.ImageThumb,
	pluginv1.ImageKind_IMAGE_KIND_BANNER:     core.ImageBanner,
	pluginv1.ImageKind_IMAGE_KIND_ART:        core.ImageArt,
	pluginv1.ImageKind_IMAGE_KIND_DISC:       core.ImageDisc,
	pluginv1.ImageKind_IMAGE_KIND_SCREENSHOT: core.ImageScreenshot,
}
