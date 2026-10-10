// Package didl writes DIDL-Lite, the XML in which ContentDirectory
// describes objects and AVTransport receives the metadata of what it
// plays.
package didl

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UPnP classes of objects.
const (
	ClassStorageFolder = "object.container.storageFolder"
	ClassMusicAlbum    = "object.container.album.musicAlbum"
	ClassPhotoAlbum    = "object.container.album.photoAlbum"
	ClassMusicArtist   = "object.container.person.musicArtist"
	ClassPlaylist      = "object.container.playlistContainer"
	ClassVideo         = "object.item.videoItem"
	ClassMovie         = "object.item.videoItem.movie"
	ClassMusicVideo    = "object.item.videoItem.musicVideoClip"
	ClassMusicTrack    = "object.item.audioItem.musicTrack"
	ClassAudioBook     = "object.item.audioItem.audioBook"
	ClassPhoto         = "object.item.imageItem.photo"
)

// Object is a container or an item.
type Object struct {
	ID, ParentID string
	Container    bool
	// ChildCount of a container; negative leaves it out.
	ChildCount int
	Searchable bool
	Title      string
	Class      string
	// Date is the object's date, written as YYYY-MM-DD.
	Date        time.Time
	Description string
	Genres      []string
	Artists     []string
	AlbumArtist string
	Album       string
	// TrackNumber is the original track or episode number; 0 leaves it
	// out.
	TrackNumber int
	// AlbumArt is the URL of the object's artwork, in JPEG.
	AlbumArt string
	// AlbumArtProfile is the DLNA profile of AlbumArt, e.g. "JPEG_TN".
	AlbumArtProfile string
	Res             []Res
	// Caption is the URL of the subtitles of a video for renderers that
	// read Samsung's sec:CaptionInfoEx; Res can carry them too.
	Caption, CaptionType string
}

// Res is a resource of an item: a URL and what it holds.
type Res struct {
	URL          string
	ProtocolInfo string
	Size         int64
	Duration     time.Duration
	// Resolution is "WIDTHxHEIGHT".
	Resolution      string
	Bitrate         int64 // bytes per second, as UPnP counts it
	SampleFrequency int
	AudioChannels   int
}

// Header and Footer enclose DIDL-Lite objects.
const (
	Header = `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" ` +
		`xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/" ` +
		`xmlns:sec="http://www.sec.co.kr/">`
	Footer = `</DIDL-Lite>`
)

// Marshal returns objects as a DIDL-Lite document.
func Marshal(objects []Object) string {
	var b strings.Builder
	b.WriteString(Header)
	for i := range objects {
		objects[i].write(&b)
	}
	b.WriteString(Footer)
	return b.String()
}

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (o *Object) write(b *strings.Builder) {
	tag := "item"
	if o.Container {
		tag = "container"
	}
	fmt.Fprintf(b, `<%s id="%s" parentID="%s" restricted="1"`, tag, esc(o.ID), esc(o.ParentID))
	if o.Container {
		if o.ChildCount >= 0 {
			fmt.Fprintf(b, ` childCount="%d"`, o.ChildCount)
		}
		fmt.Fprintf(b, ` searchable="%d"`, boolInt(o.Searchable))
	}
	b.WriteString(">")
	elem(b, "dc:title", o.Title)
	elem(b, "upnp:class", o.Class)
	if !o.Date.IsZero() {
		elem(b, "dc:date", o.Date.Format(time.DateOnly))
	}
	elem(b, "dc:description", o.Description)
	for _, g := range o.Genres {
		elem(b, "upnp:genre", g)
	}
	for _, a := range o.Artists {
		elem(b, "upnp:artist", a)
		elem(b, "dc:creator", a)
	}
	if o.AlbumArtist != "" {
		fmt.Fprintf(b, `<upnp:artist role="AlbumArtist">%s</upnp:artist>`, esc(o.AlbumArtist))
	}
	elem(b, "upnp:album", o.Album)
	if o.TrackNumber > 0 {
		elem(b, "upnp:originalTrackNumber", strconv.Itoa(o.TrackNumber))
	}
	if o.AlbumArt != "" {
		if o.AlbumArtProfile != "" {
			fmt.Fprintf(b, `<upnp:albumArtURI dlna:profileID="%s">%s</upnp:albumArtURI>`, esc(o.AlbumArtProfile), esc(o.AlbumArt))
		} else {
			elem(b, "upnp:albumArtURI", o.AlbumArt)
		}
	}
	for _, r := range o.Res {
		r.write(b)
	}
	if o.Caption != "" {
		fmt.Fprintf(b, `<sec:CaptionInfoEx sec:type="%s">%s</sec:CaptionInfoEx>`, esc(o.CaptionType), esc(o.Caption))
	}
	fmt.Fprintf(b, "</%s>", tag)
}

func (r *Res) write(b *strings.Builder) {
	fmt.Fprintf(b, `<res protocolInfo="%s"`, esc(r.ProtocolInfo))
	if r.Size > 0 {
		fmt.Fprintf(b, ` size="%d"`, r.Size)
	}
	if r.Duration > 0 {
		fmt.Fprintf(b, ` duration="%s"`, Duration(r.Duration))
	}
	if r.Resolution != "" {
		fmt.Fprintf(b, ` resolution="%s"`, esc(r.Resolution))
	}
	if r.Bitrate > 0 {
		fmt.Fprintf(b, ` bitrate="%d"`, r.Bitrate)
	}
	if r.SampleFrequency > 0 {
		fmt.Fprintf(b, ` sampleFrequency="%d"`, r.SampleFrequency)
	}
	if r.AudioChannels > 0 {
		fmt.Fprintf(b, ` nrAudioChannels="%d"`, r.AudioChannels)
	}
	fmt.Fprintf(b, ">%s</res>", esc(r.URL))
}

func elem(b *strings.Builder, name, value string) {
	if value != "" {
		fmt.Fprintf(b, "<%s>%s</%s>", name, esc(value), name)
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// Duration formats a duration as DIDL-Lite and AVTransport write them,
// H+:MM:SS.FFF.
func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	return fmt.Sprintf("%d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
}

// ParseDuration reads H+:MM:SS[.F+] or H+:MM:SS[.F0/F1]; "NOT_IMPLEMENTED"
// and other values that are not durations are errors.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("duration %q", s)
	}
	h, err1 := strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
	m, err2 := strconv.Atoi(parts[1])
	sec, frac, _ := strings.Cut(parts[2], ".")
	sv, err3 := strconv.Atoi(sec)
	if err := cmpErr(err1, err2, err3); err != nil || h < 0 || m < 0 || m > 59 || sv < 0 || sv > 59 {
		return 0, fmt.Errorf("duration %q", s)
	}
	d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sv)*time.Second
	if num, den, ok := strings.Cut(frac, "/"); ok {
		n, err1 := strconv.Atoi(num)
		dv, err2 := strconv.Atoi(den)
		if cmpErr(err1, err2) != nil || dv <= 0 {
			return 0, fmt.Errorf("duration %q", s)
		}
		d += time.Duration(n) * time.Second / time.Duration(dv)
	} else if frac != "" {
		f, err := strconv.ParseFloat("0."+frac, 64)
		if err != nil {
			return 0, fmt.Errorf("duration %q", s)
		}
		d += time.Duration(f * float64(time.Second))
	}
	return d, nil
}

func cmpErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
