package planner

import (
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
)

func TestCropBlackBorders(t *testing.T) {
	video := core.MediaStream{
		Kind: core.StreamVideo, Index: 0, Codec: "h264", Width: 1920, Height: 1080, BitDepth: 8, PixelFormat: "yuv420p",
		RealFrameRate: core.Rational{Num: 24, Den: 1}, Crop: &core.Crop{Top: 140, Bottom: 140},
	}
	job := func(edit func(*Job)) *Job {
		src := &decision.Source{MediaSource: &core.MediaSource{Container: "mkv", Streams: []core.MediaStream{video}}}
		j := &Job{
			Delivery: HLS, Source: src, Video: &src.Streams[0], Request: &decision.Decision{},
			VideoCodec: "h264", MaxWidth: 1280, MaxHeight: 720,
		}
		if edit != nil {
			edit(j)
		}
		return j
	}
	p := &Planner{Options: DefaultOptions()}
	graph := func(p *Planner, j *Job) string { return p.SoftwareFilters(j, false).Main.String() }

	// The borders go before scaling, and the output keeps the picture's
	// 2.4:1.
	g := graph(p, job(nil))
	crop, scale := strings.Index(g, "crop=w=1920:h=800:x=0:y=140"), strings.Index(g, "scale=")
	if crop < 0 || scale < crop {
		t.Errorf("filters = %s, want the crop before scaling", g)
	}
	if o := p.Output(job(nil)); o.Width != 1280 || o.Height != 532 {
		t.Errorf("output = %dx%d, want 1280x532", o.Width, o.Height)
	}

	off := &Planner{Options: DefaultOptions()}
	off.Options.CropBlackBorders = false
	for name, tt := range map[string]struct {
		p *Planner
		j *Job
	}{
		"copied video":        {p, job(func(j *Job) { j.VideoCodec = Copy })},
		"subtitles burned in": {p, job(func(j *Job) { j.SubtitleMethod = decision.SubtitleEncode })},
		"rotated":             {p, job(func(j *Job) { j.Video.Rotation = 90 })},
		"no borders found":    {p, job(func(j *Job) { j.Video.Crop = &core.Crop{} })},
		"not looked for":      {p, job(func(j *Job) { j.Video.Crop = nil })},
		"option off":          {off, job(nil)},
	} {
		if g := graph(tt.p, tt.j); strings.Contains(g, "crop=") {
			t.Errorf("%s: filters = %s, want no crop", name, g)
		}
	}
}
