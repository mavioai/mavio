package hwaccel

import (
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestEncoderValidatorCases(t *testing.T) {
	portedCases(t, "encoder_validator.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ValidateVersionInternalTest": func(t *testing.T, a args) {
				err := CheckVersion(a.constant(t, "versionOutput"))
				if got, want := err == nil, a.boolean(t, "valid"); got != want {
					t.Errorf("valid: got = %v (%v), want = %v", got, err, want)
				}
			},
		},
	})
}

// TestParseVersion ports GetFFmpegVersionTest, whose ClassData testport
// cannot extract.
func TestParseVersion(t *testing.T) {
	tests := []struct {
		symbol string
		want   Version
	}{
		{"FFmpegV701Output", NewVersion(7, 0, 1)},
		{"FFmpegV611Output", NewVersion(6, 1, 1)},
		{"FFmpegV60Output", NewVersion(6, 0)},
		{"FFmpegV512Output", NewVersion(5, 1, 2)},
		{"FFmpegV44Output", NewVersion(4, 4)},
		{"FFmpegV432Output", NewVersion(4, 3, 2)},
		{"FFmpegGitUnknownOutput2", NewVersion(4, 4)},
		{"FFmpegGitWithoutLibpostprocOutput", NewVersion(4, 4)},
		{"FFmpegGitUnknownOutput", Version{}},
	}
	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			if got := ParseVersion(constant(t, "EncoderValidatorTestsData."+tt.symbol)); got != tt.want {
				t.Errorf("got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"4.4", "4.4", 0},
		{"4.4", "4.4.0", -1},
		{"4.10", "4.9", 1},
		{"7.0.1", "6.1.1", 1},
		{"5.1.2.3", "5.1.2", 1},
	}
	for _, tt := range tests {
		a, ok1 := ParseVersionString(tt.a)
		b, ok2 := ParseVersionString(tt.b)
		if !ok1 || !ok2 {
			t.Fatalf("parse %s, %s failed", tt.a, tt.b)
		}
		if got := a.Compare(b); got != tt.want {
			t.Errorf("Compare(%s, %s): got = %d, want = %d", tt.a, tt.b, got, tt.want)
		}
		if a.String() != tt.a {
			t.Errorf("String: got = %s, want = %s", a, tt.a)
		}
	}
	for _, s := range []string{"", "7", "4.", "1.2.3.4.5", "a.b", "-1.2", "+1.2"} {
		if v, ok := ParseVersionString(s); ok {
			t.Errorf("ParseVersionString(%q): got = %v, want = failure", s, v)
		}
	}
}

func TestCheckVersionAvconv(t *testing.T) {
	if err := CheckVersion("avconv version 12, Copyright (c) 2000-2016 the Libav developers"); err == nil {
		t.Error("avconv accepted")
	}
}

const encodersOutput = `Encoders:
 V..... = Video
 ------
 V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 V....D h264_videotoolbox    VideoToolbox H.264 Encoder (codec h264)
 V....D libwebp              libwebp WebP image (codec webp)
 A....D aac                  AAC (Advanced Audio Coding)
 A....D libopus              libopus Opus (codec opus)
`

const filtersOutput = `Filters:
  T.. = Timeline support
  ------
 ... alphasrc          |->V       Generate a video source with alpha.
 TSC zscale            V->V       Apply resizing, colorspace and bit depth conversion.
 ... scale_vt          V->V       Scale Videotoolbox frames
 T.C drawtext          V->V       Draw text on top of video frames using libfreetype library.
`

func TestParseListings(t *testing.T) {
	if got, want := ParseCodecs(encodersOutput, true), []string{"libx264", "h264_videotoolbox", "aac", "libopus"}; !slices.Equal(got, want) {
		t.Errorf("encoders: got = %v, want = %v", got, want)
	}
	if got, want := ParseCodecs(encodersOutput, false), []string{"aac"}; !slices.Equal(got, want) {
		t.Errorf("decoders: got = %v, want = %v", got, want)
	}
	if got, want := ParseFilters(filtersOutput), []string{"alphasrc", "zscale", "scale_vt"}; !slices.Equal(got, want) {
		t.Errorf("filters: got = %v, want = %v", got, want)
	}
	hw := "Hardware acceleration methods:\r\nvideotoolbox\r\n\r\nvaapi\nvideotoolbox\n"
	if got, want := ParseHwaccels(hw), []string{"videotoolbox", "vaapi"}; !slices.Equal(got, want) {
		t.Errorf("hwaccels: got = %v, want = %v", got, want)
	}
}

func TestCapabilities(t *testing.T) {
	c := &Capabilities{Encoders: []string{"libopus", "h264_nvenc"}, Hwaccels: []string{"cuda"}}
	if !c.SupportsEncoder("H264_NVENC") || !c.SupportsHwaccel("CUDA") || c.SupportsDecoder("h264") {
		t.Error("Supports*: case-insensitive lookup failed")
	}
	if !c.CanEncodeAudio("Opus") || c.CanEncodeAudio("mp3") {
		t.Error("CanEncodeAudio: got = wrong mapping")
	}
}

func TestHasAV1HardwareDecode(t *testing.T) {
	tests := []struct {
		arch, brand string
		want        bool
	}{
		{"arm64", "Apple M1 Max", false},
		{"arm64", "Apple M2", false},
		{"arm64", "Apple M3 Pro", true},
		{"arm64", "Apple M4", true},
		{"amd64", "Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz", false},
	}
	for _, tt := range tests {
		if got := HasAV1HardwareDecode(tt.arch, tt.brand); got != tt.want {
			t.Errorf("HasAV1HardwareDecode(%s, %s): got = %v, want = %v", tt.arch, tt.brand, got, tt.want)
		}
	}
}

func TestApplePlatformHelperCases(t *testing.T) {
	portedCases(t, "encoder/apple_platform_helper.json", ported{
		facts: map[string]string{"GetSysctlValue_CpuBrand_NotEmpty": "TestCPUBrand"},
	})
}

func TestCPUBrand(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	brand, err := CPUBrand()
	if err != nil {
		t.Fatal(err)
	}
	if brand == "" || strings.Contains(brand, "\x00") {
		t.Errorf("got = %q, want a brand without NUL", brand)
	}
}

func TestProcessWrapperCases(t *testing.T) {
	portedCases(t, "encoder/process_wrapper.json", ported{
		skip: map[string]string{
			"ExitedProcess_StaysUsableForTheCallerThatStartedIt": "races between .NET Process exit events and their caller; exec.Cmd.Wait has no exit event",
			"ExitState_IsReadableBeforeTheExitEventArrives":      "races between .NET Process exit events and their caller; exec.Cmd.Wait has no exit event",
			"ExitCode_SurvivesDisposal":                          "races between .NET Process exit events and their caller; exec.Cmd.Wait has no exit event",
		},
	})
}

func TestDetect(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	d := &Detector{FFmpeg: ffmpeg, FFprobe: ffprobe}
	v, err := d.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	c, err := d.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != v || c.Version.IsZero() {
		t.Errorf("version: got = %v, want = %v", c.Version, v)
	}
	if !c.SupportsDecoder("h264") || !c.SupportsEncoder("aac") {
		t.Errorf("codecs: got = %v / %v", c.Decoders, c.Encoders)
	}
	if runtime.GOOS == "darwin" && !c.SupportsHwaccel("videotoolbox") {
		t.Errorf("hwaccels: got = %v, want videotoolbox", c.Hwaccels)
	}
	t.Logf("capabilities: %+v", c)
}
