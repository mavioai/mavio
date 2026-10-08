package hwaccel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
)

// Capabilities describes what an ffmpeg build can do on this host.
type Capabilities struct {
	// Version is the ffmpeg version, see [ParseVersion].
	Version  Version
	Decoders []string
	Encoders []string
	// Hwaccels are the methods of "ffmpeg -hwaccels", such as "vaapi".
	Hwaccels      []string
	Filters       []string
	FilterOptions map[FilterOption]bool
	BSFOptions    map[BSFOption]bool
	// PauseKey reports whether the build pauses transcoding on "p" read
	// from standard input, a jellyfin-ffmpeg extension.
	PauseKey bool
	// LowPriorityHwDecode reports whether -hwaccel_flags +low_priority is
	// accepted.
	LowPriorityHwDecode bool
	// ProbeFirstVideoFrame reports whether ffprobe accepts -only_first_vframe.
	ProbeFirstVideoFrame bool
	// VAAPI describes the configured VA-API render node on Linux.
	VAAPI VAAPIDevice
	// VideoToolboxAV1 reports whether VideoToolbox decodes AV1 in hardware.
	VideoToolboxAV1 bool
}

// VAAPIDevice describes a VA-API render node.
type VAAPIDevice struct {
	AMD      bool // Mesa Gallium driver
	IntelIHD bool // Intel iHD driver
	Intel965 bool // Intel i965 driver
	// VulkanDRMModifier reports Vulkan support for DRM format modifiers.
	VulkanDRMModifier bool
	// VulkanDRMInterop reports Vulkan support for DMA-BUF import.
	VulkanDRMInterop bool
}

func hasFold(list []string, name string) bool {
	for _, s := range list {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// SupportsDecoder reports whether the named decoder is available.
func (c *Capabilities) SupportsDecoder(name string) bool { return hasFold(c.Decoders, name) }

// SupportsEncoder reports whether the named encoder is available.
func (c *Capabilities) SupportsEncoder(name string) bool { return hasFold(c.Encoders, name) }

// SupportsHwaccel reports whether the named hardware acceleration method is
// available.
func (c *Capabilities) SupportsHwaccel(name string) bool { return hasFold(c.Hwaccels, name) }

// SupportsFilter reports whether the named filter is available.
func (c *Capabilities) SupportsFilter(name string) bool { return hasFold(c.Filters, name) }

// SupportsFilterOption reports whether a filter option is available.
func (c *Capabilities) SupportsFilterOption(o FilterOption) bool { return c.FilterOptions[o] }

// SupportsBSFOption reports whether a bitstream filter option is available.
func (c *Capabilities) SupportsBSFOption(o BSFOption) bool { return c.BSFOptions[o] }

// CanEncodeAudio reports whether audio can be encoded to codec, mapping
// "opus" and "mp3" to their library encoders.
func (c *Capabilities) CanEncodeAudio(codec string) bool {
	switch strings.ToLower(codec) {
	case "opus":
		codec = "libopus"
	case "mp3":
		codec = "libmp3lame"
	}
	return c.SupportsEncoder(codec)
}

// Vulkan device extensions required for DRM format modifiers and for DMA-BUF
// interop with VA-API.
var (
	vulkanDRMModifierExts = []string{"VK_EXT_image_drm_format_modifier"}
	vulkanDRMInteropExts  = []string{
		"VK_KHR_external_memory_fd",
		"VK_EXT_external_memory_dma_buf",
		"VK_KHR_external_semaphore_fd",
		"VK_EXT_external_memory_host",
	}
)

// Detector interrogates an ffmpeg build.
type Detector struct {
	FFmpeg  string
	FFprobe string
	// VAAPIDevice is the render node to inspect, such as /dev/dri/renderD128,
	// when VA-API is the configured acceleration; empty skips the checks.
	VAAPIDevice string
	Logger      *slog.Logger
}

// Validate runs "ffmpeg -version" and checks the build with [CheckVersion].
func (d *Detector) Validate(ctx context.Context) (Version, error) {
	out, err := d.output(ctx, d.FFmpeg, false, "", "-version")
	if err != nil {
		return Version{}, err
	}
	if strings.TrimSpace(out) == "" {
		return Version{}, fmt.Errorf("%s -version: no output", d.FFmpeg)
	}
	if err := CheckVersion(out); err != nil {
		return Version{}, err
	}
	return ParseVersion(out), nil
}

// Detect lists the codecs, filters and hardware acceleration methods of the
// build and probes the options and devices the planner depends on. Failing
// probes leave their capability unset.
func (d *Detector) Detect(ctx context.Context) (*Capabilities, error) {
	c := &Capabilities{FilterOptions: map[FilterOption]bool{}, BSFOptions: map[BSFOption]bool{}}
	list := func(args ...string) string {
		out, err := d.output(ctx, d.FFmpeg, false, "", args...)
		if err != nil {
			d.logger().WarnContext(ctx, "ffmpeg listing failed", "args", args, "err", err)
		}
		return out
	}
	version, err := d.output(ctx, d.FFmpeg, false, "", "-version")
	if err != nil {
		return nil, err
	}
	c.Version = ParseVersion(version)
	c.Decoders = ParseCodecs(list("-decoders"), false)
	c.Encoders = ParseCodecs(list("-encoders"), true)
	c.Filters = ParseFilters(list("-filters"))
	c.Hwaccels = ParseHwaccels(list("-hwaccels"))
	for o, p := range filterOptionProbes {
		c.FilterOptions[o] = hasOption(list("-h", "filter="+p.filter), "Filter", p)
	}
	for o, p := range bsfOptionProbes {
		c.BSFOptions[o] = hasOption(list("-h", "bsf="+p.filter), "Bit stream filter", p)
	}

	// A dummy realtime encode prints the key bindings on "?" and stops on
	// "q"; its input lasts at most 5 seconds should "q" be ignored.
	keys, _ := d.output(ctx, d.FFmpeg, true, "?q", "-hide_banner", "-re", "-f", "lavfi", "-i", "nullsrc=s=1x1:r=1:d=5", "-f", "null", "-")
	c.PauseKey = strings.Contains(keys, "p      pause transcoding")
	c.LowPriorityHwDecode = d.succeeds(ctx, d.FFmpeg, "-loglevel", "quiet", "-hwaccel_flags", "+low_priority", "-hide_banner", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=100", "-f", "null", "-")
	if d.FFprobe != "" {
		c.ProbeFirstVideoFrame = d.succeeds(ctx, d.FFprobe, "-loglevel", "quiet", "-f", "lavfi", "-i", "nullsrc=s=1x1:d=1", "-only_first_vframe")
	}

	if runtime.GOOS == "linux" && d.VAAPIDevice != "" && c.SupportsHwaccel("vaapi") {
		drivers, _ := d.output(ctx, d.FFmpeg, true, "", "-v", "verbose", "-hide_banner", "-init_hw_device", "vaapi=va:"+d.VAAPIDevice)
		c.VAAPI.AMD = strings.Contains(drivers, "Mesa Gallium driver")
		c.VAAPI.IntelIHD = strings.Contains(drivers, "Intel iHD driver")
		c.VAAPI.Intel965 = strings.Contains(drivers, "Intel i965 driver")
		exts, _ := d.output(ctx, d.FFmpeg, true, "", "-v", "verbose", "-hide_banner", "-init_hw_device", "drm=dr:"+d.VAAPIDevice, "-init_hw_device", "vulkan=vk@dr")
		c.VAAPI.VulkanDRMModifier = containsAll(exts, vulkanDRMModifierExts)
		c.VAAPI.VulkanDRMInterop = containsAll(exts, vulkanDRMInteropExts)
	}
	if runtime.GOOS == "darwin" && c.SupportsHwaccel("videotoolbox") {
		brand, err := CPUBrand()
		if err != nil {
			d.logger().WarnContext(ctx, "reading the CPU brand failed", "err", err)
		}
		c.VideoToolboxAV1 = err == nil && HasAV1HardwareDecode(runtime.GOARCH, brand)
	}
	return c, ctx.Err()
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func (d *Detector) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// output runs a program and returns its standard output, or its standard
// error when stderr is set. Exit statuses are ignored: help and device
// probes print what is wanted and still fail.
func (d *Detector) output(ctx context.Context, path string, stderr bool, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
		return "", fmt.Errorf("run %s: %w", path, err)
	}
	if stderr {
		return errOut.String(), nil
	}
	return out.String(), nil
}

// succeeds reports whether a program exits with status 0.
func (d *Detector) succeeds(ctx context.Context, path string, args ...string) bool {
	return exec.CommandContext(ctx, path, args...).Run() == nil
}
