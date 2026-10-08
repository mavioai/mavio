package planner

import (
	"strconv"
	"strings"
)

// Filter is one ffmpeg filter with its options in order.
type Filter struct {
	Name string
	Args []Arg
	// Raw holds options that are written as is, such as a scale expression;
	// it follows Args.
	Raw string
}

// Arg is a named filter option.
type Arg struct{ Key, Value string }

// F builds a filter from alternating keys and values.
func F(name string, kv ...string) Filter {
	f := Filter{Name: name}
	for i := 0; i+1 < len(kv); i += 2 {
		f.Args = append(f.Args, Arg{kv[i], kv[i+1]})
	}
	return f
}

// Get returns the value of an option.
func (f Filter) Get(key string) (string, bool) {
	for _, a := range f.Args {
		if a.Key == key {
			return a.Value, true
		}
	}
	return "", false
}

// String formats the filter as ffmpeg expects it, e.g.
// "scale=w=1280:h=-2".
func (f Filter) String() string {
	var b strings.Builder
	b.WriteString(f.Name)
	sep := "="
	for _, a := range f.Args {
		b.WriteString(sep)
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value)
		sep = ":"
	}
	if f.Raw != "" {
		b.WriteString(sep)
		b.WriteString(f.Raw)
	}
	return b.String()
}

// Chain is a linear sequence of filters.
type Chain []Filter

// String joins the filters with commas.
func (c Chain) String() string {
	parts := make([]string, len(c))
	for i, f := range c {
		parts[i] = f.String()
	}
	return strings.Join(parts, ",")
}

// Find returns the first filter with the given name.
func (c Chain) Find(name string) (Filter, bool) {
	for _, f := range c {
		if f.Name == name {
			return f, true
		}
	}
	return Filter{}, false
}

// VideoGraph is the processing of the video stream: the main chain, and
// for graphical subtitles burned in, the chain preparing the subtitle and
// the overlay combining both.
type VideoGraph struct {
	Main     Chain
	Subtitle Chain
	Overlay  Chain
	// HWDevice names the hardware frames context the graph runs in, if any.
	HWDevice string
}

// Args returns the ffmpeg options applying the graph to video stream
// videoIndex of input 0 and, with graphical subtitles, to subtitle stream
// subIndex of input subInput.
func (g *VideoGraph) Args(videoIndex, subInput, subIndex int) []string {
	if len(g.Overlay) == 0 {
		if len(g.Main) == 0 {
			return nil
		}
		return []string{"-vf", g.Main.String()}
	}
	var parts []string
	main := "[0:" + strconv.Itoa(videoIndex) + "]"
	if len(g.Main) > 0 {
		parts = append(parts, main+g.Main.String()+"[main]")
		main = "[main]"
	}
	sub := "[" + strconv.Itoa(subInput) + ":" + strconv.Itoa(subIndex) + "]"
	if len(g.Subtitle) > 0 {
		parts = append(parts, sub+g.Subtitle.String()+"[sub]")
		sub = "[sub]"
	}
	parts = append(parts, main+sub+g.Overlay.String())
	return []string{"-filter_complex", strings.Join(parts, ";")}
}
