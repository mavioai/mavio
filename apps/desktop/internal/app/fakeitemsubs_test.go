package app

import (
	"context"
	"fmt"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

// itemSubs are the subtitles of a fake server's items, by their IDs.
func (s *fakeSubtitles) itemSubs(id string) []*acquisitionv1.ItemSubtitle {
	if s.items == nil {
		s.items = map[string][]*acquisitionv1.ItemSubtitle{"orbit": {
			acquisitionv1.ItemSubtitle_builder{
				Language: acquisitionv1.SubtitleLanguageSpec_builder{Language: new("en")}.Build(),
				Path:     new("/media/Orbit/Orbit.en.srt"), Codec: new("srt"), Counted: new(true),
			}.Build(),
			acquisitionv1.ItemSubtitle_builder{
				Language:    acquisitionv1.SubtitleLanguageSpec_builder{Language: new("de")}.Build(),
				StreamIndex: new(int32(3)), Codec: new("subrip"),
			}.Build(),
		}}
	}
	return s.items[id]
}

func (s *fakeSubtitles) GetItemSubtitles(_ context.Context, r *acquisitionv1.GetItemSubtitlesRequest) (*acquisitionv1.GetItemSubtitlesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.GetItemSubtitlesResponse_builder{
		Profile: s.profiles[0], Subtitles: s.itemSubs(r.GetItemId()),
		Missing: []*acquisitionv1.SubtitleLanguageSpec{acquisitionv1.SubtitleLanguageSpec_builder{Language: new("fr"), Forced: new(true)}.Build()},
	}.Build(), nil
}

func (s *fakeSubtitles) SearchItemSubtitles(_ context.Context, r *acquisitionv1.SearchItemSubtitlesRequest) (*acquisitionv1.SearchItemSubtitlesResponse, error) {
	return acquisitionv1.SearchItemSubtitlesResponse_builder{
		Subtitles: []*acquisitionv1.ScoredSubtitle{acquisitionv1.ScoredSubtitle_builder{
			Provider: new("opensubtitles"), SubtitleId: new("os-1"), Language: r.GetLanguages()[0],
			ReleaseNames: []string{"Orbit.2024.1080p.BluRay"}, Percent: new(int32(92)), Uploader: new("ana"), Format: new("srt"),
			NotMatched: []string{"source"},
		}.Build()},
		FailedProviders: []string{"podnapisi"},
	}.Build(), nil
}

func (s *fakeSubtitles) DownloadItemSubtitle(_ context.Context, r *acquisitionv1.DownloadItemSubtitleRequest) (*acquisitionv1.DownloadItemSubtitleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprint("download ", r.GetItemId(), " ", r.GetProvider(), "/", r.GetSubtitleId(), " ", r.GetLanguage().GetLanguage()))
	return acquisitionv1.DownloadItemSubtitleResponse_builder{}.Build(), nil
}

func (s *fakeSubtitles) SyncSubtitle(_ context.Context, r *acquisitionv1.SyncSubtitleRequest) (*acquisitionv1.SyncSubtitleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprint("sync ", r.GetPath(), " ", r.GetReferencePath()))
	return acquisitionv1.SyncSubtitleResponse_builder{OffsetMs: new(int64(-420)), Ratio: new(1.0)}.Build(), nil
}

func (s *fakeSubtitles) ExtractSubtitle(_ context.Context, r *acquisitionv1.ExtractSubtitleRequest) (*acquisitionv1.ExtractSubtitleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprint("extract ", r.GetStreamIndex()))
	return acquisitionv1.ExtractSubtitleResponse_builder{}.Build(), nil
}

func (s *fakeSubtitles) TranslateSubtitle(_ context.Context, r *acquisitionv1.TranslateSubtitleRequest) (*acquisitionv1.TranslateSubtitleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprint("translate ", r.GetPath(), " ", r.GetLanguage().GetLanguage(), " ", r.GetTranslator()))
	return acquisitionv1.TranslateSubtitleResponse_builder{}.Build(), nil
}

func (s *fakeSubtitles) DeleteSubtitle(_ context.Context, r *acquisitionv1.DeleteSubtitleRequest) (*acquisitionv1.DeleteSubtitleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "delete "+r.GetPath())
	subs := s.itemSubs(r.GetItemId())
	for i, x := range subs {
		if x.GetPath() == r.GetPath() {
			s.items[r.GetItemId()] = append(subs[:i:i], subs[i+1:]...)
			break
		}
	}
	return acquisitionv1.DeleteSubtitleResponse_builder{}.Build(), nil
}
