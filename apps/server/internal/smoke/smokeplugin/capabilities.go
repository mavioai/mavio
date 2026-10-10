package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// The capabilities the end-to-end test of every capability exercises,
// besides those of plugin.go:
//
//   - As an image provider it offers a logo, "<image_base>/logo.png", of
//     every item when "image_base" is configured.
//   - As a local metadata reader it takes the name from the first line of
//     "<media name>.title", or of the file named so without the media's
//     extension; as a saver it writes the name to "<media name>.title".
//   - As a metadata processor it adds the tag "processed".
//   - As a lyrics provider it finds synced lyrics of every track, whose
//     single line is the track's name.
//   - As a subtitle provider it finds an English SubRip subtitle of every
//     video, whose single cue is the video's name.
//   - As a resolver it leaves out entries named "skip*" and claims folders
//     holding "claim.me", each other file of which is a movie named after
//     it, extension and all; in a photos library each is a photo, in a
//     photo album named "Claimed Album".
//   - As an intro provider it plays first the items whose IDs the file
//     "intros" of its data folder lists.
//   - As an image generator it makes a 2 × 2 PNG primary image.
//   - As a media source provider it gives every item the source "Remote"
//     whose URL the file "source" of its data folder holds, if any.
//   - For password resets it appends "<user name> <PIN>" to the file
//     "resets" of its data folder.
//   - As a notifier it appends the type of every event to the file
//     "notified" of its data folder.
func init() {
	guest.Handle(pluginv1connect.NewImageProviderServiceHandler(images{}))
	guest.Handle(pluginv1connect.NewLocalMetadataServiceHandler(local{}))
	guest.Handle(pluginv1connect.NewMetadataSaverServiceHandler(local{}))
	guest.Handle(pluginv1connect.NewMetadataProcessorServiceHandler(processor{}))
	guest.Handle(pluginv1connect.NewLyricsProviderServiceHandler(lyrics{}))
	guest.Handle(pluginv1connect.NewSubtitleProviderServiceHandler(subtitles{}))
	guest.Handle(pluginv1connect.NewResolverServiceHandler(resolver{}))
	guest.Handle(pluginv1connect.NewIntroProviderServiceHandler(intros{}))
	guest.Handle(pluginv1connect.NewImageGeneratorServiceHandler(generator{}))
	guest.Handle(pluginv1connect.NewMediaSourceProviderServiceHandler(sources{}))
	guest.Handle(pluginv1connect.NewPasswordResetServiceHandler(resets{}))
}

// readData returns the words of a file of the data folder; none when it
// does not exist.
func readData(name string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(guest.DataDir(), name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return strings.Fields(string(data)), err
}

// appendData appends a line to a file of the data folder.
func appendData(name string, words ...any) error {
	f, err := os.OpenFile(filepath.Join(guest.DataDir(), name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, words...); err != nil {
		return err
	}
	return f.Close()
}

type images struct{}

func (images) GetImages(context.Context, *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	config.Lock()
	base := config.ImageBase
	config.Unlock()
	if base == "" {
		return &pluginv1.GetImagesResponse{}, nil
	}
	return pluginv1.GetImagesResponse_builder{Images: []*pluginv1.RemoteImage{pluginv1.RemoteImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_LOGO.Enum(), Url: proto.String(base + "/logo.png"),
	}.Build()}}.Build(), nil
}

type local struct{}

func (local) ReadMetadata(_ context.Context, req *pluginv1.ReadMetadataRequest) (*pluginv1.ReadMetadataResponse, error) {
	media := req.GetMediaName()
	names := []string{media + ".title", strings.TrimSuffix(media, filepath.Ext(media)) + ".title"}
	i := slices.IndexFunc(req.GetFiles(), func(f *pluginv1.LocalFile) bool { return slices.Contains(names, f.GetName()) })
	if i < 0 {
		return &pluginv1.ReadMetadataResponse{}, nil
	}
	name, _, _ := strings.Cut(string(req.GetFiles()[i].GetContent()), "\n")
	return pluginv1.ReadMetadataResponse_builder{
		Found: proto.Bool(true), Metadata: pluginv1.Metadata_builder{Name: &name}.Build(),
	}.Build(), nil
}

func (local) SaveMetadata(_ context.Context, req *pluginv1.SaveMetadataRequest) (*pluginv1.SaveMetadataResponse, error) {
	return pluginv1.SaveMetadataResponse_builder{Files: []*pluginv1.LocalFile{pluginv1.LocalFile_builder{
		Name: proto.String(req.GetMediaName() + ".title"), Content: []byte(req.GetMetadata().GetName() + "\n"),
	}.Build()}}.Build(), nil
}

type processor struct{}

func (processor) ProcessMetadata(_ context.Context, req *pluginv1.ProcessMetadataRequest) (*pluginv1.ProcessMetadataResponse, error) {
	if slices.Contains(req.GetMetadata().GetTags(), "processed") {
		return &pluginv1.ProcessMetadataResponse{}, nil
	}
	md := pluginv1.Metadata_builder{Tags: append(req.GetMetadata().GetTags(), "processed")}.Build()
	return pluginv1.ProcessMetadataResponse_builder{Changed: proto.Bool(true), Metadata: md}.Build(), nil
}

type lyrics struct{}

func (lyrics) SearchLyrics(_ context.Context, req *pluginv1.SearchLyricsRequest) (*pluginv1.SearchLyricsResponse, error) {
	return pluginv1.SearchLyricsResponse_builder{Lyrics: []*pluginv1.RemoteLyrics{pluginv1.RemoteLyrics_builder{
		Id: proto.String(req.GetName()), Name: proto.String(req.GetName()), Synced: proto.Bool(true),
	}.Build()}}.Build(), nil
}

func (lyrics) DownloadLyrics(_ context.Context, req *pluginv1.DownloadLyricsRequest) (*pluginv1.DownloadLyricsResponse, error) {
	return pluginv1.DownloadLyricsResponse_builder{Content: proto.String("[00:01.00]" + req.GetId() + "\n"), Synced: proto.Bool(true)}.Build(), nil
}

type subtitles struct{}

func (subtitles) SearchSubtitles(_ context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	return pluginv1.SearchSubtitlesResponse_builder{Subtitles: []*pluginv1.RemoteSubtitle{pluginv1.RemoteSubtitle_builder{
		Id: proto.String(req.GetQuery().GetName()), Name: proto.String(req.GetQuery().GetName()), Language: proto.String("en"),
		Format: proto.String("srt"),
	}.Build()}}.Build(), nil
}

func (subtitles) DownloadSubtitle(_ context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error) {
	return pluginv1.DownloadSubtitleResponse_builder{
		Data: []byte("1\n00:00:01,000 --> 00:00:02,000\n" + req.GetId() + "\n"), Format: proto.String("srt"), Language: proto.String("en"),
	}.Build(), nil
}

type resolver struct{}

func (resolver) Ignore(_ context.Context, req *pluginv1.IgnoreRequest) (*pluginv1.IgnoreResponse, error) {
	var names []string
	for _, e := range req.GetFolder().GetEntries() {
		if strings.HasPrefix(e.GetName(), "skip") {
			names = append(names, e.GetName())
		}
	}
	return pluginv1.IgnoreResponse_builder{Names: names}.Build(), nil
}

func (resolver) Resolve(_ context.Context, req *pluginv1.ResolveRequest) (*pluginv1.ResolveResponse, error) {
	entries := req.GetFolder().GetEntries()
	if !slices.ContainsFunc(entries, func(e *pluginv1.FolderEntry) bool { return e.GetName() == "claim.me" }) {
		return &pluginv1.ResolveResponse{}, nil
	}
	resp := pluginv1.ResolveResponse_builder{Claimed: proto.Bool(true)}
	kind := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if req.GetFolder().GetLibraryKind() == pluginv1.LibraryKind_LIBRARY_KIND_PHOTOS {
		kind = pluginv1.MediaKind_MEDIA_KIND_PHOTO
		resp.FolderItem = pluginv1.ResolvedItem_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_PHOTO_ALBUM.Enum(), Name: proto.String("Claimed Album"),
		}.Build()
	}
	for _, e := range entries {
		if e.GetName() != "claim.me" && !e.GetIsDir() {
			resp.Items = append(resp.Items, pluginv1.ResolvedItem_builder{
				Kind: kind.Enum(), Entry: proto.String(e.GetName()), Name: proto.String(e.GetName()),
			}.Build())
		}
	}
	return resp.Build(), nil
}

type intros struct{}

func (intros) GetIntros(context.Context, *pluginv1.GetIntrosRequest) (*pluginv1.GetIntrosResponse, error) {
	ids, err := readData("intros")
	if err != nil {
		return nil, err
	}
	return pluginv1.GetIntrosResponse_builder{ItemIds: ids}.Build(), nil
}

type generator struct{}

func (generator) GenerateImages(_ context.Context, req *pluginv1.GenerateImagesRequest) (*pluginv1.GenerateImagesResponse, error) {
	if !slices.Contains(req.GetImageKinds(), pluginv1.ImageKind_IMAGE_KIND_PRIMARY) {
		return &pluginv1.GenerateImagesResponse{}, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		return nil, err
	}
	return pluginv1.GenerateImagesResponse_builder{Images: []*pluginv1.GeneratedImage{pluginv1.GeneratedImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Content: buf.Bytes(),
	}.Build()}}.Build(), nil
}

type sources struct{}

func (sources) GetMediaSources(context.Context, *pluginv1.GetMediaSourcesRequest) (*pluginv1.GetMediaSourcesResponse, error) {
	urls, err := readData("source")
	if err != nil || len(urls) == 0 {
		return &pluginv1.GetMediaSourcesResponse{}, err
	}
	return pluginv1.GetMediaSourcesResponse_builder{Sources: []*pluginv1.RemoteMediaSource{pluginv1.RemoteMediaSource_builder{
		Id: proto.String("remote"), Name: proto.String("Remote"), Url: proto.String(urls[0]),
	}.Build()}}.Build(), nil
}

type resets struct{}

func (resets) StartReset(_ context.Context, req *pluginv1.StartResetRequest) (*pluginv1.StartResetResponse, error) {
	if err := appendData("resets", req.GetUserName(), req.GetPin()); err != nil {
		return nil, err
	}
	return &pluginv1.StartResetResponse{}, nil
}
