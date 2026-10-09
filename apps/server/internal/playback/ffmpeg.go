package playback

import (
	"context"
	"log/slog"

	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

// UseFFmpeg makes the manager transcode with the ffmpeg build at ffmpeg,
// with the hardware acceleration this host offers. It returns the ffmpeg
// version; on error cfg is unchanged and media plays directly only.
func (cfg *Config) UseFFmpeg(ctx context.Context, ffmpeg, ffprobe string) (string, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	det := &hwaccel.Detector{FFmpeg: ffmpeg, FFprobe: ffprobe, Logger: logger}
	v, err := det.Validate(ctx)
	if err != nil {
		return "", err
	}
	caps, err := det.Detect(ctx)
	if err != nil {
		return "", err
	}
	opts := planner.DefaultOptions()
	if caps.SupportsHwaccel("videotoolbox") {
		opts.Hardware = "videotoolbox"
	}
	cfg.Builder = &decision.Builder{Transcoder: caps, Logger: logger}
	cfg.Planner = &planner.Planner{Options: opts, Caps: caps}
	cfg.FFmpeg, cfg.FFmpegPath, cfg.PauseKey = supervisor.Exec(ffmpeg), ffmpeg, caps.PauseKey
	return v.String(), nil
}
