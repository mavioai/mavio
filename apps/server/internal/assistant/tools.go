package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// call is the context of a tool call: the user it runs as and, when the
// call comes from a request, its device and what the device reported.
type call struct {
	s    *Service
	ctx  context.Context
	user core.User
	// device is the requesting device's session ID; empty when the call
	// has no device, such as a model's call after a restart.
	device string
	ask    *assistantv1.AskContext
	// act carries a client action out on the requesting device; nil when
	// the call has no device.
	act func(*assistantv1.ClientAction)
	// item is told of the item a call is about, for "it" later.
	item func(id string)
}

// answerError is a failure the user is told of: the fast path speaks its
// grammar sentence, the model reads its message.
type answerError struct {
	key  string
	args map[string]string
	msg  string
}

func (e *answerError) Error() string { return e.msg }

func failure(key string, args map[string]string, format string, a ...any) error {
	return &answerError{key: key, args: args, msg: fmt.Sprintf(format, a...)}
}

// tool is one of the assistant's tools.
type tool struct {
	name, description string
	// params and result are JSON Schemas of the arguments and the result.
	params, result map[string]any
	// confirm, when set, returns the question asked before the call runs.
	confirm func(c *call, args json.RawMessage, language string) (string, error)
	run     func(c *call, args json.RawMessage) (any, error)
}

// def defines a tool whose arguments decode into A.
func def[A any](name, description string, params, result map[string]any, run func(*call, A) (any, error)) *tool {
	return &tool{
		name: name, description: description, params: params, result: result,
		run: func(c *call, raw json.RawMessage) (any, error) {
			var a A
			if len(raw) > 0 && string(raw) != "null" {
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, fmt.Errorf("arguments: %w", err)
				}
			}
			return run(c, a)
		},
	}
}

// Schema helpers.
func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(desc string, enum ...string) map[string]any {
	m := map[string]any{"type": "string", "description": desc}
	if len(enum) > 0 {
		m["enum"] = enum
	}
	return m
}

func num(desc string) map[string]any  { return map[string]any{"type": "number", "description": desc} }
func boolean(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }
func integer(d string) map[string]any { return map[string]any{"type": "integer", "description": d} }
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func arrOf(item map[string]any) map[string]any { return map[string]any{"type": "array", "items": item} }

var deviceParam = str(`The device's name or ID from list_devices; empty or "here" is the requesting device.`)

var itemSchema = obj(map[string]any{
	"id": str("ID"), "name": str("Name"), "kind": str("Kind"), "year": integer("Production year"),
	"series": str("Series of an episode or season"), "season": integer("Season number"), "episode": integer("Episode number"),
	"rating": num("Community rating, 0–10"), "played": boolean("Watched"), "favorite": boolean("A favorite"),
	"position_seconds": num("Resume position"), "runtime_seconds": num("Runtime"),
})

// tools defines the assistant's tools (docs/assistant.md §5.1).
func tools() []*tool {
	request := def("request_title", "Requests a movie or series the server lacks from the server's discovery, under the user's request permissions. Asks the user to confirm first.",
		obj(map[string]any{
			"query": str("The title"), "kind": str("movie or series; empty for either", "movie", "series", ""),
			"year": integer("The year, to tell titles apart"),
		}, "query"),
		obj(map[string]any{"requested": str("The title requested")}),
		(*call).requestTitle)
	request.confirm = func(c *call, raw json.RawMessage, language string) (string, error) {
		var a requestArgs
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", err
		}
		t, err := c.discover(a)
		if err != nil {
			return "", err
		}
		return c.s.reply(c.ctx, language, "confirm_request", map[string]string{"title": discoveredTitle(t)}), nil
	}
	return []*tool{
		def("search_library", "Finds items of the user's libraries. Every filter is optional; results come in the order asked, at most limit (default 20, at most 50).",
			obj(map[string]any{
				"query":      str("Text in names"),
				"kinds":      map[string]any{"type": "array", "items": str("", kindNames()...)},
				"genres":     strs("Genres, any of them"),
				"year_from":  integer("Earliest production year"),
				"year_to":    integer("Latest production year"),
				"person":     str("A person's name: actor, director, writer…"),
				"played":     boolean("Only watched (true) or unwatched (false) items"),
				"favorite":   boolean("Only favorites"),
				"resumable":  boolean("Only items in progress"),
				"sort":       str("Order", "name", "added", "released", "year", "rating", "runtime", "random", "last_played", "play_count"),
				"descending": boolean("Reverse the order"),
				"limit":      integer("Most results, up to 50"),
			}),
			obj(map[string]any{"items": arrOf(itemSchema), "total": integer("Matching items")}),
			(*call).searchLibrary),
		def("get_item", "An item's details: names, overview, genres, people, numbering, runtime, audio and subtitle streams, the user's data and media segments.",
			obj(map[string]any{"id": str("The item's ID")}, "id"),
			obj(map[string]any{"item": itemSchema}),
			(*call).getItem),
		def("list_next_up", "The next episodes of the series the user is watching.",
			obj(map[string]any{"series_id": str("Only this series"), "limit": integer("Most results, up to 50")}),
			obj(map[string]any{"items": arrOf(itemSchema)}),
			(*call).listNextUp),
		def("list_resume", "Items the user has started and not finished, most recently played first.",
			obj(map[string]any{"limit": integer("Most results, up to 50")}),
			obj(map[string]any{"items": arrOf(itemSchema)}),
			(*call).listResume),
		def("get_now_playing", "What a device plays: the item, the position, whether it is paused, and on the requesting device its streams and speed.",
			obj(map[string]any{"device": deviceParam}),
			obj(map[string]any{"item": itemSchema, "position_seconds": num(""), "paused": boolean(""), "device": str("")}),
			(*call).getNowPlaying),
		def("list_devices", "The user's online devices and remote devices, with what each plays.",
			obj(map[string]any{}),
			obj(map[string]any{"devices": arrOf(obj(map[string]any{
				"id": str(""), "name": str(""), "client": str(""), "this": boolean("The requesting device"), "playing": str(""), "paused": boolean(""),
			}))}),
			(*call).listDevices),
		def("read_subtitles", "Reads an item's subtitles before a position, to answer questions about the story: what was said, who said it, what happened. Never reads past the position the user has reached, so it does not spoil what is ahead.",
			obj(map[string]any{
				"item_id":          str("The item; empty is what the requesting device plays"),
				"position_seconds": num("Read up to this time; default the position the user has reached"),
				"before_seconds":   num("How far back to read, default 300, at most 1800"),
				"language":         str("ISO 639-1; default the subtitles shown, else the user's language"),
			}),
			obj(map[string]any{
				"item": str(""), "language": str(""), "from_seconds": num(""), "to_seconds": num(""),
				"reached_seconds": num("The position the user has reached; nothing after it is read"),
				"lines":           arrOf(obj(map[string]any{"start_seconds": num(""), "text": str("")})),
			}),
			(*call).readSubtitles),
		def("play", "Plays items on a device: movies, episodes, a series (its next episode), a season, albums or tracks, as a queue, shuffled or not, from the resume position, the beginning or a time.",
			obj(map[string]any{
				"item_ids":          strs("The items, in order"),
				"shuffle":           boolean("Shuffle the queue"),
				"start":             str("Where to start: resume (default) or beginning", "resume", "beginning"),
				"position_seconds":  num("Start at this time instead"),
				"device":            deviceParam,
				"audio_language":    str("Audio language on the requesting device, ISO 639-1, or original"),
				"subtitle_language": str("Subtitle language on the requesting device, ISO 639-1, or off"),
			}, "item_ids"),
			obj(map[string]any{"playing": str("What plays"), "device": str("Where"), "queue": integer("Items queued")}),
			(*call).play),
		def("control", "Controls playback on a device. Other devices take pause, resume, stop, seek, seek_by, next, previous, volume, mute and unmute only.",
			obj(map[string]any{
				"command": str("What to do", "pause", "resume", "stop", "seek", "seek_by", "skip", "next", "previous", "speed", "volume", "mute", "unmute", "fullscreen"),
				"value":   num("seek: the position in seconds; seek_by: seconds, negative backwards; speed: the rate; volume: 0–100"),
				"segment": str("skip: the media segment", "intro", "outro", "recap", "preview", "commercial"),
				"device":  deviceParam,
			}, "command"),
			obj(map[string]any{"done": str("")}),
			(*call).control),
		def("set_streams", "Changes the audio and subtitles on the requesting device, by language or stream index, turns subtitles off, or moves them in time.",
			obj(map[string]any{
				"audio_language":          str("ISO 639-1, or original"),
				"audio_index":             integer("Audio stream index from get_item"),
				"subtitle_language":       str("ISO 639-1"),
				"subtitle_index":          integer("Subtitle stream index from get_item"),
				"subtitles_off":           boolean("Turn subtitles off"),
				"subtitles_on":            boolean("Turn subtitles on in the best language"),
				"subtitle_offset_seconds": num("Move subtitles in time: positive later, negative earlier"),
			}),
			obj(map[string]any{"audio": str(""), "subtitles": str("")}),
			(*call).setStreams),
		def("set_user_data", "Marks an item watched or unwatched, a favorite or not, or rates it.",
			obj(map[string]any{
				"item_id": str("The item"), "played": boolean("Watched"), "favorite": boolean("A favorite"),
				"rating": num("The user's rating, 0–10"),
			}, "item_id"),
			obj(map[string]any{"item": itemSchema}),
			(*call).setUserData),
		def("add_to_playlist", "Adds items to one of the user's playlists by name, made when missing.",
			obj(map[string]any{"playlist": str("The playlist's name"), "item_ids": strs("The items")}, "playlist", "item_ids"),
			obj(map[string]any{"playlist": str(""), "created": boolean("")}),
			(*call).addToPlaylist),
		request,
		def("navigate", "Opens a screen on the requesting device.",
			obj(map[string]any{
				"target": str("The screen", "home", "back", "item", "library", "search", "settings"),
				"id":     str("item: the item's ID; library: the library's ID; settings: the section"),
				"query":  str("search: the text"),
			}, "target"),
			obj(map[string]any{"done": str("")}),
			(*call).navigate),
	}
}

// Kinds.
var kinds = map[string]libraryv1.ItemKind{
	"movie": libraryv1.ItemKind_ITEM_KIND_MOVIE, "series": libraryv1.ItemKind_ITEM_KIND_SERIES,
	"season": libraryv1.ItemKind_ITEM_KIND_SEASON, "episode": libraryv1.ItemKind_ITEM_KIND_EPISODE,
	"video": libraryv1.ItemKind_ITEM_KIND_VIDEO, "artist": libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST,
	"album": libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM, "track": libraryv1.ItemKind_ITEM_KIND_TRACK,
	"music_video": libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, "audiobook": libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK,
	"collection": libraryv1.ItemKind_ITEM_KIND_COLLECTION, "playlist": libraryv1.ItemKind_ITEM_KIND_PLAYLIST,
}

func kindNames() []string {
	out := make([]string, 0, len(kinds))
	for k := range kinds {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func kindName(k libraryv1.ItemKind) string {
	for n, v := range kinds {
		if v == k {
			return n
		}
	}
	return strings.ToLower(strings.TrimPrefix(k.String(), "ITEM_KIND_"))
}

var sorts = map[string]libraryv1.SortField{
	"name": libraryv1.SortField_SORT_FIELD_NAME, "added": libraryv1.SortField_SORT_FIELD_DATE_ADDED,
	"released": libraryv1.SortField_SORT_FIELD_PREMIERE_DATE, "year": libraryv1.SortField_SORT_FIELD_PRODUCTION_YEAR,
	"rating": libraryv1.SortField_SORT_FIELD_COMMUNITY_RATING, "runtime": libraryv1.SortField_SORT_FIELD_RUNTIME,
	"random": libraryv1.SortField_SORT_FIELD_RANDOM, "last_played": libraryv1.SortField_SORT_FIELD_LAST_PLAYED,
	"play_count": libraryv1.SortField_SORT_FIELD_PLAY_COUNT,
}

// itemSummary is an item as tools return it.
type itemSummary struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Year     int32   `json:"year,omitempty"`
	Series   string  `json:"series,omitempty"`
	Season   int32   `json:"season,omitempty"`
	Episode  int32   `json:"episode,omitempty"`
	Rating   float64 `json:"rating,omitempty"`
	Played   bool    `json:"played,omitempty"`
	Favorite bool    `json:"favorite,omitempty"`
	Position float64 `json:"position_seconds,omitempty"`
	Runtime  float64 `json:"runtime_seconds,omitempty"`
}

func summarize(it *libraryv1.Item) itemSummary {
	s := itemSummary{
		ID: it.GetId(), Name: it.GetName(), Kind: kindName(it.GetKind()), Year: it.GetProductionYear(),
		Series: it.GetSeriesName(), Rating: it.GetCommunityRating(), Runtime: it.GetRuntime().AsDuration().Seconds(),
	}
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_EPISODE:
		s.Season, s.Episode = it.GetParentIndexNumber(), it.GetIndexNumber()
	case libraryv1.ItemKind_ITEM_KIND_SEASON:
		s.Season = it.GetIndexNumber()
	}
	if ud := it.GetUserData(); ud != nil {
		s.Played, s.Favorite, s.Position = ud.GetPlayed(), ud.GetFavorite(), ud.GetPosition().AsDuration().Seconds()
	}
	return s
}

func summaries(items []*libraryv1.Item) []itemSummary {
	out := make([]itemSummary, 0, len(items))
	for _, it := range items {
		out = append(out, summarize(it))
	}
	return out
}

// title names an item as answers say it.
func title(it *libraryv1.Item) string {
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_EPISODE, libraryv1.ItemKind_ITEM_KIND_SEASON:
		if s := it.GetSeriesName(); s != "" && s != it.GetName() {
			return s + ": " + it.GetName()
		}
	case libraryv1.ItemKind_ITEM_KIND_TRACK:
		if a := it.GetArtists(); len(a) > 0 {
			return it.GetName() + " – " + a[0]
		}
	}
	return it.GetName()
}

func limit(n, def int) int32 {
	if n <= 0 {
		return int32(def)
	}
	return int32(min(n, 50))
}

type searchArgs struct {
	Query      string   `json:"query"`
	Kinds      []string `json:"kinds"`
	Genres     []string `json:"genres"`
	YearFrom   int32    `json:"year_from"`
	YearTo     int32    `json:"year_to"`
	Person     string   `json:"person"`
	Played     *bool    `json:"played"`
	Favorite   *bool    `json:"favorite"`
	Resumable  bool     `json:"resumable"`
	Sort       string   `json:"sort"`
	Descending bool     `json:"descending"`
	Limit      int      `json:"limit"`
}

type itemsResult struct {
	Items []itemSummary `json:"items"`
	Total int32         `json:"total,omitempty"`
	items []*libraryv1.Item
}

func (c *call) searchLibrary(a searchArgs) (any, error) {
	req := &libraryv1.ListItemsRequest{}
	req.SetSearch(strings.TrimSpace(a.Query))
	var ks []libraryv1.ItemKind
	for _, k := range a.Kinds {
		if v, ok := kinds[k]; ok {
			ks = append(ks, v)
		}
	}
	if len(ks) == 0 && a.Query == "" && a.Person == "" {
		ks = []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES}
	}
	req.SetKinds(ks)
	req.SetGenres(a.Genres)
	if a.YearFrom > 0 {
		req.SetYearFrom(a.YearFrom)
	}
	if a.YearTo > 0 {
		req.SetYearTo(a.YearTo)
	}
	if a.Played != nil {
		req.SetPlayed(*a.Played)
	}
	if a.Favorite != nil {
		req.SetFavorite(*a.Favorite)
	}
	if a.Resumable {
		req.SetResumable(true)
	}
	if a.Person != "" {
		p, err := c.person(a.Person)
		if err != nil {
			return nil, err
		}
		req.SetPersonId(p.GetId())
	}
	if f, ok := sorts[a.Sort]; ok {
		req.SetSort([]*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: &f, Descending: &a.Descending}.Build()})
	}
	req.SetLimit(limit(a.Limit, 20))
	resp, err := c.s.cfg.Services.Items.ListItems(c.ctx, req)
	if err != nil {
		return nil, err
	}
	return itemsResult{Items: summaries(resp.GetItems()), Total: resp.GetTotal(), items: resp.GetItems()}, nil
}

// person finds a person of the user's libraries by name.
func (c *call) person(name string) (*libraryv1.Person, error) {
	resp, err := c.s.cfg.Services.Items.ListPeople(c.ctx, libraryv1.ListPeopleRequest_builder{Search: &name, Limit: new(int32(10))}.Build())
	if err != nil {
		return nil, err
	}
	people := resp.GetPeople()
	for _, p := range people {
		if strings.EqualFold(p.GetPerson().GetName(), name) {
			return p.GetPerson(), nil
		}
	}
	if len(people) == 0 {
		return nil, failure("no_title", map[string]string{"title": name}, "no person called %q", name)
	}
	return people[0].GetPerson(), nil
}

type idArgs struct {
	ID string `json:"id"`
}

type itemDetails struct {
	itemSummary
	OriginalTitle string       `json:"original_title,omitempty"`
	Overview      string       `json:"overview,omitempty"`
	Genres        []string     `json:"genres,omitempty"`
	Studios       []string     `json:"studios,omitempty"`
	Rated         string       `json:"official_rating,omitempty"`
	SeriesID      string       `json:"series_id,omitempty"`
	People        []personRole `json:"people,omitempty"`
	Audio         []streamInfo `json:"audio,omitempty"`
	Subtitles     []streamInfo `json:"subtitles,omitempty"`
	Segments      []string     `json:"segments,omitempty"`
}

type personRole struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Role string `json:"role,omitempty"`
}

type streamInfo struct {
	Index    int32  `json:"index"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

func (c *call) getItem(a idArgs) (any, error) {
	resp, err := c.s.cfg.Services.Items.GetItem(c.ctx, libraryv1.GetItemRequest_builder{Id: &a.ID}.Build())
	if err != nil {
		return nil, err
	}
	it := resp.GetItem()
	if c.item != nil {
		c.item(it.GetId())
	}
	d := itemDetails{
		itemSummary: summarize(it), OriginalTitle: it.GetOriginalTitle(), Overview: it.GetOverview(),
		Genres: it.GetGenres(), Studios: it.GetStudios(), Rated: it.GetOfficialRating(), SeriesID: it.GetSeriesId(),
	}
	for i, cr := range resp.GetCredits() {
		if i == 15 {
			break
		}
		d.People = append(d.People, personRole{
			Name: cr.GetPerson().GetName(), Kind: strings.ToLower(strings.TrimPrefix(cr.GetKind().String(), "CREDIT_KIND_")), Role: cr.GetRole(),
		})
	}
	if ms := resp.GetMediaSources(); len(ms) > 0 {
		for _, st := range ms[0].GetStreams() {
			info := streamInfo{Index: st.GetIndex(), Language: twoLetter(st.GetLanguage()), Title: st.GetTitle()}
			switch st.GetKind() {
			case libraryv1.StreamKind_STREAM_KIND_AUDIO:
				d.Audio = append(d.Audio, info)
			case libraryv1.StreamKind_STREAM_KIND_SUBTITLE:
				d.Subtitles = append(d.Subtitles, info)
			}
		}
	}
	for _, seg := range resp.GetSegments() {
		d.Segments = append(d.Segments, strings.ToLower(strings.TrimPrefix(seg.GetKind().String(), "SEGMENT_KIND_")))
	}
	return map[string]any{"item": d}, nil
}

// twoLetter converts a stream's language to ISO 639-1 where it has one.
func twoLetter(lang string) string {
	if l, ok := metadata.FindLanguage(lang); ok && l.TwoLetterCode != "" {
		b, _, _ := strings.Cut(l.TwoLetterCode, "-")
		return b
	}
	return lang
}

type listArgs struct {
	SeriesID string `json:"series_id"`
	Limit    int    `json:"limit"`
}

func (c *call) listNextUp(a listArgs) (any, error) {
	req := &libraryv1.ListNextUpRequest{}
	if a.SeriesID != "" {
		req.SetSeriesId(a.SeriesID)
	}
	req.SetLimit(limit(a.Limit, 10))
	resp, err := c.s.cfg.Services.Items.ListNextUp(c.ctx, req)
	if err != nil {
		return nil, err
	}
	return itemsResult{Items: summaries(resp.GetItems()), items: resp.GetItems()}, nil
}

func (c *call) listResume(a listArgs) (any, error) {
	field, desc := libraryv1.SortField_SORT_FIELD_LAST_PLAYED, true
	resp, err := c.s.cfg.Services.Items.ListItems(c.ctx, libraryv1.ListItemsRequest_builder{
		Resumable: new(true), Limit: new(limit(a.Limit, 10)),
		Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: &field, Descending: &desc}.Build()},
	}.Build())
	if err != nil {
		return nil, err
	}
	return itemsResult{Items: summaries(resp.GetItems()), items: resp.GetItems()}, nil
}

// here reports whether device names the requesting device.
func (c *call) here(device string) bool {
	d := strings.ToLower(strings.TrimSpace(device))
	return d == "" || d == "here" || d == "this" || d == "this device" || (c.device != "" && device == c.device)
}

// target is a device a call acts on: the requesting one, or another
// session.
type target struct {
	here    bool
	session *sessionv1.Session
}

func (t target) name() string {
	if t.session == nil {
		return ""
	}
	return t.session.GetDeviceName()
}

// sessions lists the user's sessions and remote devices.
func (c *call) sessions() ([]*sessionv1.Session, error) {
	resp, err := c.s.cfg.Services.Sessions.ListSessions(c.ctx, &sessionv1.ListSessionsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetSessions(), nil
}

// target resolves a device's name or ID.
func (c *call) target(device string) (target, error) {
	if c.here(device) {
		if c.act == nil {
			return target{}, failure("no_device", map[string]string{"device": "here"}, "the request has no device; name one from list_devices")
		}
		return target{here: true}, nil
	}
	list, err := c.sessions()
	if err != nil {
		return target{}, err
	}
	want := normalize(device)
	var best *sessionv1.Session
	for _, s := range list {
		if !s.GetOnline() || s.GetCurrent() {
			continue
		}
		name := normalize(s.GetDeviceName())
		switch {
		case s.GetId() == device || name == want:
			return target{session: s}, nil
		case best == nil && (strings.Contains(name, want) || strings.Contains(want, name)):
			best = s
		}
	}
	if best == nil {
		return target{}, failure("no_device", map[string]string{"device": device}, "no online device called %q", device)
	}
	return target{session: best}, nil
}

// send sends a command to another device.
func (c *call) send(t target, cmd *sessionv1.Command) error {
	_, err := c.s.cfg.Services.Sessions.SendCommand(c.ctx, sessionv1.SendCommandRequest_builder{
		SessionId: new(t.session.GetId()), Command: cmd,
	}.Build())
	return err
}

func (c *call) action(a *assistantv1.ClientAction) {
	a.SetId(core.NewID().String())
	c.act(a)
}

type nowPlaying struct {
	Item     *itemSummary `json:"item,omitempty"`
	Position float64      `json:"position_seconds"`
	Paused   bool         `json:"paused"`
	Speed    float64      `json:"speed,omitempty"`
	Device   string       `json:"device,omitempty"`
	item     *libraryv1.Item
}

type deviceArgs struct {
	Device string `json:"device"`
}

func (c *call) getNowPlaying(a deviceArgs) (any, error) {
	return c.nowPlaying(a.Device)
}

func (c *call) nowPlaying(device string) (nowPlaying, error) {
	if c.here(device) {
		p := c.ask.GetPlaying()
		if p.GetItemId() == "" {
			return nowPlaying{}, failure("nothing_playing", nil, "nothing plays on the requesting device")
		}
		it, err := c.fetch(p.GetItemId())
		if err != nil {
			return nowPlaying{}, err
		}
		s := summarize(it)
		return nowPlaying{Item: &s, item: it, Position: p.GetPosition().AsDuration().Seconds(), Paused: p.GetPaused(), Speed: p.GetSpeed()}, nil
	}
	t, err := c.target(device)
	if err != nil {
		return nowPlaying{}, err
	}
	np := t.session.GetNowPlaying()
	if np == nil {
		return nowPlaying{}, failure("nothing_playing", nil, "nothing plays on %s", t.name())
	}
	s := summarize(np.GetItem())
	if c.item != nil {
		c.item(np.GetItem().GetId())
	}
	return nowPlaying{
		Item: &s, item: np.GetItem(), Position: np.GetPosition().AsDuration().Seconds(), Paused: np.GetPaused(), Device: t.name(),
	}, nil
}

// fetch returns an item and tells the call of it.
func (c *call) fetch(id string) (*libraryv1.Item, error) {
	resp, err := c.s.cfg.Services.Items.GetItem(c.ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
	if err != nil {
		return nil, err
	}
	if c.item != nil {
		c.item(id)
	}
	return resp.GetItem(), nil
}

type deviceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Client  string `json:"client,omitempty"`
	This    bool   `json:"this,omitempty"`
	Playing string `json:"playing,omitempty"`
	Paused  bool   `json:"paused,omitempty"`
}

func (c *call) listDevices(struct{}) (any, error) {
	list, err := c.sessions()
	if err != nil {
		return nil, err
	}
	var out []deviceInfo
	for _, s := range list {
		if !s.GetOnline() && !s.GetCurrent() {
			continue
		}
		d := deviceInfo{ID: s.GetId(), Name: s.GetDeviceName(), Client: s.GetClient(), This: s.GetCurrent()}
		if np := s.GetNowPlaying(); np != nil {
			d.Playing, d.Paused = title(np.GetItem()), np.GetPaused()
		}
		out = append(out, d)
	}
	return map[string]any{"devices": out}, nil
}

type playArgs struct {
	ItemIDs          []string `json:"item_ids"`
	Shuffle          bool     `json:"shuffle"`
	Start            string   `json:"start"`
	Position         *float64 `json:"position_seconds"`
	Device           string   `json:"device"`
	AudioLanguage    string   `json:"audio_language"`
	SubtitleLanguage string   `json:"subtitle_language"`
}

type playResult struct {
	Playing string `json:"playing"`
	Device  string `json:"device,omitempty"`
	Queue   int    `json:"queue"`
	first   *libraryv1.Item
}

var playable = []libraryv1.ItemKind{
	libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_EPISODE, libraryv1.ItemKind_ITEM_KIND_VIDEO,
	libraryv1.ItemKind_ITEM_KIND_TRACK, libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK,
}

// expand turns an item into what plays: a series' next episode, a
// season's episodes from the first unwatched, a container's playable
// items.
func (c *call) expand(it *libraryv1.Item) ([]*libraryv1.Item, error) {
	items := c.s.cfg.Services.Items
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_SERIES:
		next, err := items.ListNextUp(c.ctx, libraryv1.ListNextUpRequest_builder{SeriesId: new(it.GetId()), IncludeResumable: new(true), Limit: new(int32(1))}.Build())
		if err != nil {
			return nil, err
		}
		if len(next.GetItems()) > 0 {
			return next.GetItems(), nil
		}
		return c.children(it.GetId(), []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, 1)
	case libraryv1.ItemKind_ITEM_KIND_SEASON:
		eps, err := c.children(it.GetId(), []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, 1000)
		if err != nil {
			return nil, err
		}
		if i := slices.IndexFunc(eps, func(e *libraryv1.Item) bool { return !e.GetUserData().GetPlayed() }); i > 0 {
			eps = eps[i:]
		}
		return eps, nil
	default:
		if slices.Contains(playable, it.GetKind()) {
			return []*libraryv1.Item{it}, nil
		}
		return c.children(it.GetId(), playable, 1000)
	}
}

func (c *call) children(parent string, ks []libraryv1.ItemKind, n int32) ([]*libraryv1.Item, error) {
	field := libraryv1.SortField_SORT_FIELD_INDEX
	resp, err := c.s.cfg.Services.Items.ListItems(c.ctx, libraryv1.ListItemsRequest_builder{
		ParentId: &parent, Recursive: new(true), Kinds: ks, Limit: &n,
		Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: &field}.Build()},
	}.Build())
	if err != nil {
		return nil, err
	}
	return resp.GetItems(), nil
}

func (c *call) play(a playArgs) (any, error) {
	if len(a.ItemIDs) == 0 {
		return nil, errors.New("item_ids is empty")
	}
	t, err := c.target(a.Device)
	if err != nil {
		return nil, err
	}
	var queue []*libraryv1.Item
	for _, id := range a.ItemIDs {
		it, err := c.fetch(id)
		if err != nil {
			return nil, err
		}
		more, err := c.expand(it)
		if err != nil {
			return nil, err
		}
		queue = append(queue, more...)
	}
	if len(queue) == 0 {
		return nil, failure("no_title", map[string]string{"title": ""}, "nothing to play there")
	}
	if a.Shuffle {
		rand.Shuffle(len(queue), func(i, j int) { queue[i], queue[j] = queue[j], queue[i] })
	}
	first := queue[0]
	var start time.Duration
	switch {
	case a.Position != nil:
		start = time.Duration(*a.Position * float64(time.Second))
	case a.Start != "beginning" && !first.GetUserData().GetPlayed():
		start = first.GetUserData().GetPosition().AsDuration()
	}
	ids := make([]string, 0, len(queue))
	for _, it := range queue {
		ids = append(ids, it.GetId())
	}
	if c.item != nil {
		c.item(first.GetId())
	}
	res := playResult{Playing: title(first), Device: t.name(), Queue: len(queue), first: first}
	if !t.here {
		return res, c.send(t, sessionv1.Command_builder{Play: sessionv1.Play_builder{
			ItemIds: ids, StartPosition: durationpb.New(start),
		}.Build()}.Build())
	}
	act := assistantv1.PlayAction_builder{ItemIds: ids, StartPosition: durationpb.New(start)}.Build()
	if a.AudioLanguage != "" || a.SubtitleLanguage != "" {
		streams, err := c.streamsOf(first.GetId())
		if err != nil {
			return nil, err
		}
		if a.AudioLanguage != "" {
			if i, ok := streams.audio(a.AudioLanguage, first); ok {
				act.SetAudioStreamIndex(i)
			}
		}
		switch a.SubtitleLanguage {
		case "":
		case "off":
			act.SetSubtitleStreamIndex(-1)
		default:
			if i, ok := streams.subtitle(a.SubtitleLanguage); ok {
				act.SetSubtitleStreamIndex(i)
			}
		}
	}
	c.action(assistantv1.ClientAction_builder{Play: act}.Build())
	c.s.record(c.ctx, &c.user, "assistant.played", c.user.Name+" played "+title(first)+" through the assistant", parseID(first.GetId()), nil)
	return res, nil
}

func parseID(s string) core.ID {
	id, _ := core.ParseID(s)
	return id
}

// streams are an item's audio and subtitle streams.
type streams struct {
	audios, subtitles []*libraryv1.MediaStream
	original          string
}

func (c *call) streamsOf(id string) (streams, error) {
	resp, err := c.s.cfg.Services.Items.GetItem(c.ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
	if err != nil {
		return streams{}, err
	}
	var s streams
	if ms := resp.GetMediaSources(); len(ms) > 0 {
		for _, st := range ms[0].GetStreams() {
			switch st.GetKind() {
			case libraryv1.StreamKind_STREAM_KIND_AUDIO:
				s.audios = append(s.audios, st)
			case libraryv1.StreamKind_STREAM_KIND_SUBTITLE:
				s.subtitles = append(s.subtitles, st)
			}
		}
	}
	s.original = resp.GetItem().GetOriginalLanguage()
	if s.original == "" && resp.GetItem().GetSeriesId() != "" {
		if series, err := c.s.cfg.Services.Items.GetItem(c.ctx, libraryv1.GetItemRequest_builder{Id: new(resp.GetItem().GetSeriesId())}.Build()); err == nil {
			s.original = series.GetItem().GetOriginalLanguage()
		}
	}
	return s, nil
}

func sameLanguage(stream, want string) bool {
	if stream == "" || want == "" {
		return false
	}
	return strings.EqualFold(twoLetter(stream), twoLetter(want))
}

// audio finds the audio stream of a language, "original" the item's
// original language's or else the default one.
func (s streams) audio(lang string, _ *libraryv1.Item) (int32, bool) {
	if lang == "original" {
		if s.original != "" {
			if i, ok := s.audio(s.original, nil); ok {
				return i, true
			}
		}
		for _, st := range s.audios {
			if st.GetDefault() {
				return st.GetIndex(), true
			}
		}
		if len(s.audios) > 0 {
			return s.audios[0].GetIndex(), true
		}
		return 0, false
	}
	for _, st := range s.audios {
		if sameLanguage(st.GetLanguage(), lang) {
			return st.GetIndex(), true
		}
	}
	return 0, false
}

// subtitle finds the subtitle of a language, preferring full subtitles to
// forced ones.
func (s streams) subtitle(lang string) (int32, bool) {
	var forced *libraryv1.MediaStream
	for _, st := range s.subtitles {
		if !sameLanguage(st.GetLanguage(), lang) {
			continue
		}
		if !st.GetForced() {
			return st.GetIndex(), true
		}
		forced = st
	}
	if forced != nil {
		return forced.GetIndex(), true
	}
	return 0, false
}

type subtitlesArgs struct {
	ItemID   string   `json:"item_id"`
	Position *float64 `json:"position_seconds"`
	Before   float64  `json:"before_seconds"`
	Language string   `json:"language"`
}

type subtitleLine struct {
	Start float64 `json:"start_seconds"`
	Text  string  `json:"text"`
}

type subtitlesResult struct {
	Item     string         `json:"item"`
	Language string         `json:"language,omitempty"`
	From     float64        `json:"from_seconds"`
	To       float64        `json:"to_seconds"`
	Reached  float64        `json:"reached_seconds"`
	Lines    []subtitleLine `json:"lines"`
}

// readSubtitles reads an item's subtitles up to a position, never past
// the position the user has reached: the user data's, or the requesting
// device's when it plays the item further on; a watched item is read
// whole (docs/assistant.md §5.1).
func (c *call) readSubtitles(a subtitlesArgs) (any, error) {
	if c.s.cfg.Services.Subtitles == nil {
		return nil, errors.New("the server reads no subtitles")
	}
	playing := c.ask.GetPlaying()
	id := a.ItemID
	if id == "" {
		if id = playing.GetItemId(); id == "" {
			return nil, failure("nothing_playing", nil, "nothing plays on the requesting device; name the item")
		}
	}
	it, err := c.fetch(id)
	if err != nil {
		return nil, err
	}
	reached := it.GetUserData().GetPosition().AsDuration()
	if playing.GetItemId() == it.GetId() {
		reached = max(reached, playing.GetPosition().AsDuration())
	}
	at := reached
	watched := it.GetUserData().GetPlayed()
	if watched {
		reached = max(reached, it.GetRuntime().AsDuration())
		if at == 0 {
			at = reached
		}
	}
	if a.Position != nil {
		at = time.Duration(max(*a.Position, 0) * float64(time.Second))
	}
	if watched {
		reached = max(reached, at)
	}
	at = min(at, reached)
	if at <= 0 {
		return nil, fmt.Errorf("the user has not started %s; its subtitles are not read before they watch it", title(it))
	}
	before := 300 * time.Second
	if a.Before > 0 {
		before = time.Duration(min(a.Before, 1800) * float64(time.Second))
	}
	from := max(at-before, 0)
	st, err := c.streamsOf(it.GetId())
	if err != nil {
		return nil, err
	}
	sub, stream, err := c.subtitleOf(it, st, a.Language)
	if err != nil {
		return nil, err
	}
	res := subtitlesResult{
		Item: title(it), Language: twoLetter(stream.GetLanguage()),
		From: from.Seconds(), To: at.Seconds(), Reached: reached.Seconds(), Lines: []subtitleLine{},
	}
	for _, cue := range sub.Cues {
		if cue.End <= from || cue.Start >= at {
			continue
		}
		text := strings.Join(strings.Fields(subtitle.PlainText(cue.Text)), " ")
		if text == "" {
			continue
		}
		res.Lines = append(res.Lines, subtitleLine{Start: cue.Start.Seconds(), Text: text})
	}
	return res, nil
}

// subtitleOf reads the subtitle stream of an item in a language, else the
// one the requesting device shows, else one in the user's languages, else
// any it can read.
func (c *call) subtitleOf(it *libraryv1.Item, st streams, lang string) (*subtitle.Subtitle, *libraryv1.MediaStream, error) {
	var order []int32
	if lang != "" {
		i, ok := st.subtitle(lang)
		if !ok {
			return nil, nil, failure("no_subtitles", map[string]string{"language": lang}, "no %s subtitles", lang)
		}
		order = append(order, i)
	} else {
		if p := c.ask.GetPlaying(); p.GetItemId() == it.GetId() && p.GetSubtitleStreamIndex() >= 0 && p.HasSubtitleStreamIndex() {
			order = append(order, p.GetSubtitleStreamIndex())
		}
		for _, l := range c.ask.GetLanguages() {
			if i, ok := st.subtitle(l); ok {
				order = append(order, i)
			}
		}
		for _, s := range st.subtitles {
			if !s.GetForced() {
				order = append(order, s.GetIndex())
			}
		}
	}
	last := failure("no_subtitles", map[string]string{"language": lang}, "%s has no subtitles to read", title(it))
	tried := map[int32]bool{}
	for _, i := range order {
		if tried[i] {
			continue
		}
		tried[i] = true
		sub, err := c.s.cfg.Services.Subtitles(c.ctx, parseID(it.GetId()), int(i))
		if err != nil {
			last = err
			continue
		}
		var stream *libraryv1.MediaStream
		for _, s := range st.subtitles {
			if s.GetIndex() == i {
				stream = s
			}
		}
		return sub, stream, nil
	}
	return nil, nil, last
}

type controlArgs struct {
	Command string  `json:"command"`
	Value   float64 `json:"value"`
	Segment string  `json:"segment"`
	Device  string  `json:"device"`
}

var controls = map[string]assistantv1.ControlCommand{
	"pause": assistantv1.ControlCommand_CONTROL_COMMAND_PAUSE, "resume": assistantv1.ControlCommand_CONTROL_COMMAND_RESUME,
	"stop": assistantv1.ControlCommand_CONTROL_COMMAND_STOP, "seek": assistantv1.ControlCommand_CONTROL_COMMAND_SEEK,
	"seek_by": assistantv1.ControlCommand_CONTROL_COMMAND_SEEK_BY, "skip": assistantv1.ControlCommand_CONTROL_COMMAND_SKIP,
	"next": assistantv1.ControlCommand_CONTROL_COMMAND_NEXT, "previous": assistantv1.ControlCommand_CONTROL_COMMAND_PREVIOUS,
	"speed": assistantv1.ControlCommand_CONTROL_COMMAND_SPEED, "volume": assistantv1.ControlCommand_CONTROL_COMMAND_VOLUME,
	"mute": assistantv1.ControlCommand_CONTROL_COMMAND_MUTE, "unmute": assistantv1.ControlCommand_CONTROL_COMMAND_UNMUTE,
	"fullscreen": assistantv1.ControlCommand_CONTROL_COMMAND_FULLSCREEN,
}

var playStates = map[string]sessionv1.PlayStateCommand{
	"pause": sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE, "resume": sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_UNPAUSE,
	"stop": sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_STOP, "next": sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_NEXT,
	"previous": sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PREVIOUS,
}

func (c *call) control(a controlArgs) (any, error) {
	cmd, ok := controls[a.Command]
	if !ok {
		return nil, fmt.Errorf("unknown command %q", a.Command)
	}
	t, err := c.target(a.Device)
	if err != nil {
		return nil, err
	}
	done := map[string]any{"done": a.Command}
	if t.here {
		if a.Command == "skip" && a.Segment == "" {
			a.Segment = "intro"
		}
		c.action(assistantv1.ClientAction_builder{Control: assistantv1.ControlAction_builder{
			Command: &cmd, Value: &a.Value, Segment: &a.Segment,
		}.Build()}.Build())
		return done, nil
	}
	if t.session.GetNowPlaying() == nil && a.Command != "volume" && a.Command != "mute" && a.Command != "unmute" {
		return nil, failure("nothing_playing", nil, "nothing plays on %s", t.name())
	}
	var send *sessionv1.Command
	switch a.Command {
	case "seek", "seek_by":
		pos := a.Value
		if a.Command == "seek_by" {
			pos += t.session.GetNowPlaying().GetPosition().AsDuration().Seconds()
		}
		send = sessionv1.Command_builder{Seek: sessionv1.Seek_builder{
			Position: durationpb.New(time.Duration(max(pos, 0) * float64(time.Second))),
		}.Build()}.Build()
	case "volume":
		send = sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Level: new(int32(min(max(a.Value, 0), 100)))}.Build()}.Build()
	case "mute", "unmute":
		send = sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Muted: new(a.Command == "mute")}.Build()}.Build()
	default:
		ps, ok := playStates[a.Command]
		if !ok {
			return nil, failure("not_allowed", nil, "%s cannot %s from another device", t.name(), a.Command)
		}
		send = sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{Command: &ps}.Build()}.Build()
	}
	done["device"] = t.name()
	return done, c.send(t, send)
}

type streamsArgs struct {
	AudioLanguage    string   `json:"audio_language"`
	AudioIndex       *int32   `json:"audio_index"`
	SubtitleLanguage string   `json:"subtitle_language"`
	SubtitleIndex    *int32   `json:"subtitle_index"`
	SubtitlesOff     bool     `json:"subtitles_off"`
	SubtitlesOn      bool     `json:"subtitles_on"`
	SubtitleOffset   *float64 `json:"subtitle_offset_seconds"`
}

func (c *call) setStreams(a streamsArgs) (any, error) {
	if c.act == nil {
		return nil, failure("no_device", map[string]string{"device": "here"}, "streams change only on the requesting device")
	}
	item := c.ask.GetPlaying().GetItemId()
	if item == "" {
		return nil, failure("nothing_playing", nil, "nothing plays on the requesting device")
	}
	act := &assistantv1.StreamsAction{}
	res := map[string]any{}
	var st streams
	if a.AudioLanguage != "" || a.SubtitleLanguage != "" || a.SubtitlesOn {
		var err error
		if st, err = c.streamsOf(item); err != nil {
			return nil, err
		}
	}
	switch {
	case a.AudioIndex != nil:
		act.SetAudioStreamIndex(*a.AudioIndex)
	case a.AudioLanguage != "":
		i, ok := st.audio(a.AudioLanguage, nil)
		if !ok {
			return nil, failure("no_audio", map[string]string{"language": a.AudioLanguage}, "no %s audio", a.AudioLanguage)
		}
		act.SetAudioStreamIndex(i)
		res["audio"] = a.AudioLanguage
	}
	switch {
	case a.SubtitlesOff:
		act.SetSubtitleStreamIndex(-1)
		res["subtitles"] = "off"
	case a.SubtitleIndex != nil:
		act.SetSubtitleStreamIndex(*a.SubtitleIndex)
	case a.SubtitleLanguage != "":
		i, ok := st.subtitle(a.SubtitleLanguage)
		if !ok {
			return nil, failure("no_subtitles", map[string]string{"language": a.SubtitleLanguage}, "no %s subtitles", a.SubtitleLanguage)
		}
		act.SetSubtitleStreamIndex(i)
		res["subtitles"] = a.SubtitleLanguage
	case a.SubtitlesOn:
		if len(st.subtitles) == 0 {
			return nil, failure("no_subtitles", map[string]string{"language": ""}, "the item has no subtitles")
		}
		pick := st.subtitles[0].GetIndex()
		for _, lang := range c.ask.GetLanguages() {
			if i, ok := st.subtitle(lang); ok {
				pick = i
				break
			}
		}
		act.SetSubtitleStreamIndex(pick)
		res["subtitles"] = "on"
	}
	if a.SubtitleOffset != nil {
		act.SetSubtitleOffset(*a.SubtitleOffset)
	}
	c.action(assistantv1.ClientAction_builder{Streams: act}.Build())
	return res, nil
}

type userDataArgs struct {
	ItemID   string   `json:"item_id"`
	Played   *bool    `json:"played"`
	Favorite *bool    `json:"favorite"`
	Rating   *float64 `json:"rating"`
}

func (c *call) setUserData(a userDataArgs) (any, error) {
	req := userv1.UpdateUserDataRequest_builder{ItemId: &a.ItemID, Played: a.Played, Favorite: a.Favorite, Rating: a.Rating}.Build()
	if _, err := c.s.cfg.Services.UserData.UpdateUserData(c.ctx, req); err != nil {
		return nil, err
	}
	it, err := c.fetch(a.ItemID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"item": summarize(it)}, nil
}

type playlistArgs struct {
	Playlist string   `json:"playlist"`
	ItemIDs  []string `json:"item_ids"`
}

func (c *call) addToPlaylist(a playlistArgs) (any, error) {
	pl := c.s.cfg.Services.Playlists
	list, err := pl.ListPlaylists(c.ctx, libraryv1.ListPlaylistsRequest_builder{Limit: new(int32(1000))}.Build())
	if err != nil {
		return nil, err
	}
	want := normalize(a.Playlist)
	for _, p := range list.GetPlaylists() {
		if normalize(p.GetName()) == want {
			_, err := pl.AddToPlaylist(c.ctx, libraryv1.AddToPlaylistRequest_builder{Id: new(p.GetId()), ItemIds: a.ItemIDs}.Build())
			return map[string]any{"playlist": p.GetName(), "created": false}, err
		}
	}
	if strings.TrimSpace(a.Playlist) == "" {
		return nil, failure("no_playlist", map[string]string{"playlist": ""}, "name the playlist")
	}
	_, err = pl.CreatePlaylist(c.ctx, libraryv1.CreatePlaylistRequest_builder{Name: new(strings.TrimSpace(a.Playlist)), ItemIds: a.ItemIDs}.Build())
	return map[string]any{"playlist": a.Playlist, "created": true}, err
}

type requestArgs struct {
	Query string `json:"query"`
	Kind  string `json:"kind"`
	Year  int32  `json:"year"`
}

func (c *call) discover(a requestArgs) (*acquisitionv1.DiscoveredTitle, error) {
	if c.s.cfg.Services.Discover == nil || c.s.cfg.Services.Requests == nil {
		return nil, failure("not_allowed", nil, "the server takes no requests")
	}
	req := acquisitionv1.SearchTitlesRequest_builder{Query: &a.Query}.Build()
	switch a.Kind {
	case "movie":
		req.SetMediaKind(acquisitionv1.MediaKind_MEDIA_KIND_MOVIE)
	case "series":
		req.SetMediaKind(acquisitionv1.MediaKind_MEDIA_KIND_SERIES)
	}
	if a.Year > 0 {
		req.SetYear(a.Year)
	}
	resp, err := c.s.cfg.Services.Discover.SearchTitles(c.ctx, req)
	if err != nil {
		return nil, err
	}
	for _, t := range resp.GetTitles() {
		if t.GetExternalIds()["tmdb"] != "" {
			return t, nil
		}
	}
	return nil, failure("no_title", map[string]string{"title": a.Query}, "found no title %q to request", a.Query)
}

func discoveredTitle(t *acquisitionv1.DiscoveredTitle) string {
	if t.GetYear() > 0 {
		return fmt.Sprintf("%s (%d)", t.GetName(), t.GetYear())
	}
	return t.GetName()
}

func (c *call) requestTitle(a requestArgs) (any, error) {
	t, err := c.discover(a)
	if err != nil {
		return nil, err
	}
	var tmdb int32
	if _, err := fmt.Sscan(t.GetExternalIds()["tmdb"], &tmdb); err != nil {
		return nil, fmt.Errorf("TMDB ID %q: %w", t.GetExternalIds()["tmdb"], err)
	}
	req := acquisitionv1.CreateRequestRequest_builder{MediaKind: new(t.GetMediaKind()), TmdbId: &tmdb}.Build()
	if t.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		req.SetAllSeasons(true)
	}
	if _, err := c.s.cfg.Services.Requests.CreateRequest(c.ctx, req); err != nil {
		return nil, err
	}
	return map[string]any{"requested": discoveredTitle(t)}, nil
}

type navigateArgs struct {
	Target string `json:"target"`
	ID     string `json:"id"`
	Query  string `json:"query"`
}

var navigateTargets = map[string]assistantv1.NavigateTarget{
	"home": assistantv1.NavigateTarget_NAVIGATE_TARGET_HOME, "back": assistantv1.NavigateTarget_NAVIGATE_TARGET_BACK,
	"item": assistantv1.NavigateTarget_NAVIGATE_TARGET_ITEM, "library": assistantv1.NavigateTarget_NAVIGATE_TARGET_LIBRARY,
	"search": assistantv1.NavigateTarget_NAVIGATE_TARGET_SEARCH, "settings": assistantv1.NavigateTarget_NAVIGATE_TARGET_SETTINGS,
}

func (c *call) navigate(a navigateArgs) (any, error) {
	t, ok := navigateTargets[a.Target]
	if !ok {
		return nil, fmt.Errorf("unknown target %q", a.Target)
	}
	if c.act == nil {
		return nil, failure("no_device", map[string]string{"device": "here"}, "navigation needs the requesting device")
	}
	if t == assistantv1.NavigateTarget_NAVIGATE_TARGET_ITEM && c.item != nil {
		c.item(a.ID)
	}
	c.action(assistantv1.ClientAction_builder{Navigate: assistantv1.NavigateAction_builder{Target: &t, Id: &a.ID, Query: &a.Query}.Build()}.Build())
	return map[string]any{"done": a.Target}, nil
}

// runTool runs a tool with JSON arguments and encodes its result for the
// model.
func (c *call) runTool(t *tool, args json.RawMessage) *pluginv1.SubmitAgentToolResultRequest {
	out := &pluginv1.SubmitAgentToolResultRequest{}
	v, err := t.run(c, args)
	if err != nil {
		out.SetFailed(true)
		out.SetText(err.Error())
		return out
	}
	b, err := json.Marshal(v)
	if err != nil {
		out.SetFailed(true)
		out.SetText(err.Error())
		return out
	}
	out.SetText(string(b))
	out.SetResultJson(string(b))
	return out
}
