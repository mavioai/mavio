package speech

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

func TestSpeaks(t *testing.T) {
	for _, tc := range []struct {
		languages []string
		language  string
		want      bool
	}{
		{nil, "ja", true},
		{[]string{"en", "zh-Hans"}, "", true},
		{[]string{"en", "zh-Hans"}, "zh-CN", true},
		{[]string{"en", "zh-Hans"}, "en_US", true},
		{[]string{"en", "zh-Hans"}, "ja", false},
	} {
		if got := Speaks(tc.languages, tc.language); got != tc.want {
			t.Errorf("Speaks(%v, %q) = %v, want = %v", tc.languages, tc.language, got, tc.want)
		}
	}
}

// fakePlugin lists transcribers.
type fakePlugin struct {
	pluginv1connect.SpeechToTextServiceClient
	list []*pluginv1.TranscriberInfo
}

func (f fakePlugin) ListTranscribers(context.Context, *pluginv1.ListTranscribersRequest) (*pluginv1.ListTranscribersResponse, error) {
	return pluginv1.ListTranscribersResponse_builder{Transcribers: f.list}.Build(), nil
}

func TestTranscribers(t *testing.T) {
	info := func(id string, enabled, ready bool) *pluginv1.TranscriberInfo {
		return pluginv1.TranscriberInfo_builder{Id: &id, Enabled: &enabled, Ready: &ready}.Build()
	}
	p := Plugin{ID: "dev.mavio.pipecat", Client: fakePlugin{list: []*pluginv1.TranscriberInfo{
		info("t1", true, true), info("off", false, true), info("missing", true, false), info("t2", true, true),
	}}}
	var got []string
	for _, tr := range Transcribers(t.Context(), []Plugin{p}, slog.Default()) {
		got = append(got, tr.ID)
	}
	if want := []string{"dev.mavio.pipecat/t1", "dev.mavio.pipecat/t2"}; !slices.Equal(got, want) {
		t.Errorf("Transcribers = %q, want = %q", got, want)
	}
}
