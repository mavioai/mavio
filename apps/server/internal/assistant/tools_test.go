package assistant

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

func TestReadSubtitles(t *testing.T) {
	ep := libraryv1.Item_builder{
		Id: new("e1"), Name: new("Pilot"), Kind: new(libraryv1.ItemKind_ITEM_KIND_EPISODE), Runtime: durationpb.New(40 * time.Minute),
		UserData: userv1.UserData_builder{Position: durationpb.New(10 * time.Minute)}.Build(),
	}.Build()
	watched := libraryv1.Item_builder{
		Id: new("e2"), Name: new("Finale"), Kind: new(libraryv1.ItemKind_ITEM_KIND_EPISODE), Runtime: durationpb.New(40 * time.Minute),
		UserData: userv1.UserData_builder{Played: new(true)}.Build(),
	}.Build()
	unseen := libraryv1.Item_builder{Id: new("e3"), Name: new("Next"), Kind: new(libraryv1.ItemKind_ITEM_KIND_EPISODE)}.Build()
	stream := func(i int32, lang string) *libraryv1.MediaStream {
		return libraryv1.MediaStream_builder{Index: new(i), Kind: new(libraryv1.StreamKind_STREAM_KIND_SUBTITLE), Language: new(lang)}.Build()
	}
	source := []*libraryv1.MediaSource{libraryv1.MediaSource_builder{Streams: []*libraryv1.MediaStream{stream(2, "eng"), stream(3, "chi")}}.Build()}
	// Each stream holds a cue a minute, saying its minute in its language.
	cues := func(lang string) *subtitle.Subtitle {
		var s subtitle.Subtitle
		for m := range 40 {
			s.Cues = append(s.Cues, subtitle.Cue{
				Start: time.Duration(m) * time.Minute, End: time.Duration(m)*time.Minute + 5*time.Second,
				Text: "<i>" + lang + " minute " + string(rune('A'+m)) + "</i>",
			})
		}
		return &s
	}
	s := New(Config{Services: Services{
		Items: &fakeItems{items: []*libraryv1.Item{ep, watched, unseen}, sources: map[string][]*libraryv1.MediaSource{"e1": source, "e2": source, "e3": source}},
		Subtitles: func(_ context.Context, _ core.ID, index int) (*subtitle.Subtitle, error) {
			return cues(map[int]string{2: "en", 3: "zh"}[index]), nil
		},
	}})
	read := func(playing *assistantv1.AskPlaying, args string) (subtitlesResult, error) {
		c := &call{s: s, ctx: t.Context(), ask: assistantv1.AskContext_builder{Languages: []string{"zh"}, Playing: playing}.Build()}
		v, err := s.tool["read_subtitles"].run(c, json.RawMessage(args))
		if err != nil {
			return subtitlesResult{}, err
		}
		return v.(subtitlesResult), nil
	}
	starts := func(r subtitlesResult) []float64 {
		var out []float64
		for _, l := range r.Lines {
			out = append(out, l.Start)
		}
		return out
	}
	playingAt := func(id string, at time.Duration, sub int32) *assistantv1.AskPlaying {
		return assistantv1.AskPlaying_builder{ItemId: new(id), Position: durationpb.New(at), SubtitleStreamIndex: new(sub)}.Build()
	}

	for _, tc := range []struct {
		name    string
		playing *assistantv1.AskPlaying
		args    string
		lang    string
		starts  []float64
		reached float64
	}{
		// Up to the device's position, past the saved one, in the
		// subtitles shown.
		{"playing", playingAt("e1", 12*time.Minute+30*time.Second, 2), `{"before_seconds":150}`, "en", []float64{600, 660, 720}, 750},
		// A later position is held at the one reached.
		{"ahead", playingAt("e1", 12*time.Minute+30*time.Second, 2), `{"position_seconds":2000,"before_seconds":60}`, "en", []float64{720}, 750},
		// The saved position of an item not playing, in the user's
		// language.
		{"saved", nil, `{"item_id":"e1","before_seconds":120}`, "zh", []float64{480, 540}, 600},
		// A watched item is read anywhere.
		{"watched", nil, `{"item_id":"e2","position_seconds":1810,"before_seconds":70,"language":"en"}`, "en", []float64{1740, 1800}, 2400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := read(tc.playing, tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(starts(got), tc.starts) || got.Language != tc.lang || got.Reached != tc.reached {
				t.Errorf("lines at %v in %s reached %v, want = %v in %s reached %v", starts(got), got.Language, got.Reached, tc.starts, tc.lang, tc.reached)
			}
			for _, l := range got.Lines {
				if l.Text == "" || l.Text[0] == '<' {
					t.Errorf("text = %q, want plain text", l.Text)
				}
			}
		})
	}
	if got, err := read(nil, `{"item_id":"e3"}`); err == nil {
		t.Errorf("an unstarted item read %v, want an error", got)
	}
}
