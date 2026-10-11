package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// spSetDefinition replaces the definition of a quality in e.
func spSetDefinition(e *Env, q release.Quality, minSize, maxSize float64) {
	for i := range e.Definitions {
		if e.Definitions[i].Quality == q {
			e.Definitions[i].MinSize, e.Definitions[i].MaxSize = minSize, maxSize
			return
		}
	}
	e.Definitions = append(e.Definitions, Definition{Quality: q, MinSize: minSize, MaxSize: maxSize})
}

// TestAcceptableSizeSpec ports Radarr's AcceptableSizeSpecificationFixture.
// Its SDTV definition allows 2 to 10 MB a minute.
func TestAcceptableSizeSpec(t *testing.T) {
	tests := []struct {
		name             string
		runtime          int
		size             int64
		minSize, maxSize float64
		want             bool
	}{
		{"single_episode(30, 50)", 30, 50 << 20, 2, 10, false},
		{"single_episode(30, 250)", 30, 250 << 20, 2, 10, true},
		{"single_episode(30, 500)", 30, 500 << 20, 2, 10, false},
		{"single_episode(60, 100)", 60, 100 << 20, 2, 10, false},
		{"single_episode(60, 500)", 60, 500 << 20, 2, 10, true},
		{"single_episode(60, 1000)", 60, 1000 << 20, 2, 10, false},
		{"should_return_true_if_size_is_zero", 120, 0, 10, 20, true},
		// A MaxSize of null in Radarr is 0, no limit, here.
		{"should_return_true_if_unlimited_30_minute", 30, 18457280000, 2, 0, true},
		{"should_return_true_if_unlimited_60_minute", 60, 36857280000, 2, 0, true},
		// Radarr's single test checks both sizes and that a warning was
		// logged; the warning has no equivalent here.
		{"should_use_110_minutes_if_runtime_is_0 (1095 MB)", 0, 1095 << 20, 2, 10, true},
		{"should_use_110_minutes_if_runtime_is_0 (1105 MB)", 0, 1105 << 20, 2, 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			spSetDefinition(e, release.SDTV, tt.minSize, tt.maxSize)
			c := spCandidate()
			c.Quality.Revision = release.Revision{Version: 2}
			c.Items[0].Runtime = tt.runtime
			c.Release.Size = tt.size
			spWant(t, AcceptableSize{}, e, c, nil, tt.want)
		})
	}
}

// TestFreeSpaceSpec ports Radarr's FreeSpaceSpecificationFixture. Sizes
// are in megabytes; a free space of -1 is unknown, as when the root
// folder is missing.
func TestFreeSpaceSpec(t *testing.T) {
	tests := []struct {
		name                       string
		skip                       bool
		minimum, free, releaseSize int64
		want                       bool
	}{
		{"should_return_true_when_available_space_is_more_than_size", false, 0, 200, 100, true},
		{"should_return_true_when_available_space_minus_size_is_more_than_minimum_free_space", false, 50, 200, 100, true},
		{"should_return_false_available_space_is_less_than_size", false, 0, 200, 1000, false},
		{"should_return_false_when_available_space_minus_size_is_less_than_minimum_free_space", false, 150, 200, 100, false},
		{"should_return_true_if_skip_free_space_check_is_true", true, 150, 200, 100, true},
		{"should_return_true_if_root_folder_is_not_available", false, 150, -1, 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			e.Config.SkipFreeSpaceCheck = tt.skip
			e.Config.MinimumFreeSpace = tt.minimum << 20
			if tt.free >= 0 {
				free := tt.free << 20
				ports.free = &free
			}
			c := spCandidate()
			c.Target.Path = "/test/films/movie"
			c.Release.Size = tt.releaseSize << 20
			spWant(t, FreeSpace{}, e, c, nil, tt.want)
		})
	}
}
