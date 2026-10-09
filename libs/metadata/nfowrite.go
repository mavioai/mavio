package metadata

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// nfoRoots are the root elements of the NFO files written per item kind,
// as Kodi and Jellyfin name them.
var nfoRoots = map[core.ItemKind]string{
	core.KindMovie:       "movie",
	core.KindVideo:       "movie",
	core.KindMusicVideo:  "musicvideo",
	core.KindSeries:      "tvshow",
	core.KindSeason:      "season",
	core.KindEpisode:     "episodedetails",
	core.KindMusicAlbum:  "album",
	core.KindMusicArtist: "artist",
}

// CanWriteNFO reports whether WriteNFO writes NFO files for items of kind.
func CanWriteNFO(kind core.ItemKind) bool {
	_, ok := nfoRoots[kind]
	return ok
}

// lockedFieldNames are the names NFO files record locked fields by.
var lockedFieldNames = map[core.MetadataField]string{
	core.FieldCast:                "Cast",
	core.FieldGenres:              "Genres",
	core.FieldProductionLocations: "ProductionLocations",
	core.FieldStudios:             "Studios",
	core.FieldTags:                "Tags",
	core.FieldName:                "Name",
	core.FieldOverview:            "Overview",
	core.FieldRuntime:             "Runtime",
	core.FieldOfficialRating:      "OfficialRating",
}

// creditTypes are the person types of <actor> elements.
var creditTypes = map[core.CreditKind]string{
	core.CreditActor:     "Actor",
	core.CreditGuestStar: "GuestStar",
	core.CreditDirector:  "Director",
	core.CreditWriter:    "Writer",
	core.CreditProducer:  "Producer",
	core.CreditCreator:   "Creator",
	core.CreditComposer:  "Composer",
	core.CreditConductor: "Conductor",
	core.CreditLyricist:  "Lyricist",
	core.CreditArtist:    "Artist",
	core.CreditAuthor:    "Author",
	core.CreditNarrator:  "Narrator",
	core.CreditOther:     "Unknown",
}

// imageAspects are the aspects of <thumb> elements by image kind;
// backdrops are written inside <fanart>.
var imageAspects = map[core.ImageKind]string{
	core.ImagePrimary: "poster",
	core.ImageBanner:  "banner",
	core.ImageLogo:    "clearlogo",
	core.ImageDisc:    "discart",
	core.ImageThumb:   "landscape",
	core.ImageArt:     "clearart",
}

// WriteNFO returns the NFO file of r's item, which ParseNFO reads back:
// its metadata, locks, people and remote images. Local images are not
// written, being found beside the media by their names.
func WriteNFO(r *Result) ([]byte, error) {
	it := &r.Item
	root, ok := nfoRoots[it.Kind]
	if !ok {
		return nil, fmt.Errorf("%w: no NFO files for %s items", core.ErrInvalid, it.Kind)
	}
	w := &nfoWriter{}
	w.buf.WriteString(xml.Header)
	w.enc = xml.NewEncoder(&w.buf)
	w.enc.Indent("", "  ")
	w.start(root)

	w.text("title", it.Name)
	w.text("originaltitle", it.OriginalTitle)
	w.text("sorttitle", it.SortName)
	w.text("plot", it.Overview)
	w.text("tagline", it.Tagline)
	if it.ProductionYear > 0 {
		w.text("year", strconv.Itoa(it.ProductionYear))
	}
	w.date("premiered", it.PremiereDate)
	w.date("enddate", it.EndDate)
	if it.Runtime > 0 {
		w.text("runtime", strconv.Itoa(int(it.Runtime.Round(time.Minute)/time.Minute)))
	}
	w.text("mpaa", it.OfficialRating)
	w.text("customrating", it.CustomRating)
	if it.CommunityRating > 0 {
		w.text("rating", strconv.FormatFloat(it.CommunityRating, 'f', -1, 64))
	}
	if it.CriticRating > 0 {
		w.text("criticrating", strconv.FormatFloat(it.CriticRating, 'f', -1, 64))
	}
	w.texts("genre", it.Genres)
	w.texts("tag", it.Tags)
	w.texts("studio", it.Studios)
	w.texts("country", it.ProductionLocations)
	w.text("aspectratio", it.AspectRatio)
	for _, p := range slices.Sorted(maps.Keys(it.ExternalIDs)) {
		if id := it.ExternalIDs[p]; id != "" {
			w.element("uniqueid", id, xml.Attr{Name: xml.Name{Local: "type"}, Value: string(p)})
		}
	}
	w.kind(r)
	if it.Locked {
		w.text("lockdata", "true")
	}
	if len(it.LockedFields) > 0 {
		names := make([]string, 0, len(it.LockedFields))
		for _, f := range it.LockedFields {
			if n, ok := lockedFieldNames[f]; ok {
				names = append(names, n)
			}
		}
		w.text("lockedfields", strings.Join(names, "|"))
	}
	for _, p := range r.People {
		w.start("actor")
		w.text("name", p.Name)
		w.text("role", p.Role)
		w.text("type", cmp.Or(creditTypes[p.Kind], "Unknown"))
		if p.Order != nil {
			w.text("sortorder", strconv.Itoa(*p.Order))
		}
		w.text("thumb", p.ImageURL)
		w.end("actor")
	}
	for _, img := range r.RemoteImages {
		if aspect, ok := imageAspects[img.Kind]; ok {
			w.element("thumb", img.URL, xml.Attr{Name: xml.Name{Local: "aspect"}, Value: aspect})
		}
	}
	if i := slices.IndexFunc(r.RemoteImages, func(img RemoteImage) bool { return img.Kind == core.ImageBackdrop }); i >= 0 {
		w.start("fanart")
		w.text("thumb", r.RemoteImages[i].URL)
		w.end("fanart")
	}
	w.end(root)
	if err := w.enc.Flush(); err != nil {
		return nil, err
	}
	if w.err != nil {
		return nil, w.err
	}
	w.buf.WriteByte('\n')
	return w.buf.Bytes(), nil
}

type nfoWriter struct {
	buf bytes.Buffer
	enc *xml.Encoder
	err error
}

func (w *nfoWriter) token(t xml.Token) {
	if w.err == nil {
		w.err = w.enc.EncodeToken(t)
	}
}

func (w *nfoWriter) start(name string, attrs ...xml.Attr) {
	w.token(xml.StartElement{Name: xml.Name{Local: name}, Attr: attrs})
}

func (w *nfoWriter) end(name string) { w.token(xml.EndElement{Name: xml.Name{Local: name}}) }

// element writes an element even when empty.
func (w *nfoWriter) element(name, value string, attrs ...xml.Attr) {
	w.start(name, attrs...)
	w.token(xml.CharData(value))
	w.end(name)
}

// text writes an element unless value is empty.
func (w *nfoWriter) text(name, value string) {
	if value != "" {
		w.element(name, value)
	}
}

func (w *nfoWriter) texts(name string, values []string) {
	for _, v := range values {
		w.text(name, v)
	}
}

func (w *nfoWriter) date(name string, t *time.Time) {
	if t != nil {
		w.text(name, t.Format(time.DateOnly))
	}
}

func (w *nfoWriter) number(name string, n *int) {
	if n != nil {
		w.text(name, strconv.Itoa(*n))
	}
}

// kind writes the elements specific to the item kind.
func (w *nfoWriter) kind(r *Result) {
	it := &r.Item
	switch it.Kind {
	case core.KindMovie:
		if it.CollectionName != "" {
			var attrs []xml.Attr
			if id := it.ExternalIDs[core.ProviderTMDBCollection]; id != "" {
				attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tmdbcolid"}, Value: id})
			}
			w.start("set", attrs...)
			w.text("name", it.CollectionName)
			w.end("set")
		}
	case core.KindMusicVideo:
		w.texts("artist", it.Artists)
		w.text("album", it.Album)
	case core.KindSeries:
		if it.SeriesStatus != "" {
			w.text("status", seriesStatusNames[it.SeriesStatus])
		}
		if len(it.AirDays) == 1 {
			w.text("airs_dayofweek", it.AirDays[0].String())
		} else if len(it.AirDays) == 7 {
			w.text("airs_dayofweek", "Daily")
		}
		w.text("airs_time", it.AirTime)
		w.text("displayorder", it.DisplayOrder)
	case core.KindSeason:
		w.number("seasonnumber", it.IndexNumber)
	case core.KindEpisode:
		w.text("showtitle", r.SeriesName)
		w.number("season", it.ParentIndexNumber)
		w.number("episode", it.IndexNumber)
		w.number("episodenumberend", it.IndexNumberEnd)
		w.number("airsbefore_season", it.AirsBeforeSeasonNumber)
		w.number("airsafter_season", it.AirsAfterSeasonNumber)
		w.number("airsbefore_episode", it.AirsBeforeEpisodeNumber)
	}
}

var seriesStatusNames = map[core.SeriesStatus]string{
	core.SeriesContinuing: "Continuing",
	core.SeriesEnded:      "Ended",
	core.SeriesUnreleased: "Unreleased",
}
