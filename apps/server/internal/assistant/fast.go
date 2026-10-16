package assistant

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

// slots reads an intent's slots.
type slots struct{ in *intentv1.Intent }

func (s slots) get(name string) *intentv1.Slot {
	for _, x := range s.in.GetSlots() {
		if x.GetName() == name {
			return x
		}
	}
	return nil
}

func (s slots) text(name string) string { return strings.TrimSpace(s.get(name).GetText()) }

func (s slots) keyword(name string) string { return s.get(name).GetKeyword() }

func (s slots) language(name string) string { return s.get(name).GetLanguage() }

// number reads a number, a duration or an ordinal.
func (s slots) number(name string) (float64, bool) {
	x := s.get(name)
	switch {
	case x.HasNumber():
		return x.GetNumber(), true
	case x.HasSeconds():
		return x.GetSeconds(), true
	case x.HasOrdinal():
		return float64(x.GetOrdinal()), true
	}
	return 0, false
}

// fast carries an intent out with the tools (docs/assistant.md §4).
func (t *turn) fast(in *intentv1.Intent) (outcome, error) {
	s := slots{in}
	c := t.call
	device := s.text("device")
	ctl := func(command string, value float64, key string, args map[string]string) (outcome, error) {
		if _, err := c.control(controlArgs{Command: command, Value: value, Device: device}); err != nil {
			return outcome{}, err
		}
		return outcome{key: key, args: args}, nil
	}
	playing := t.req.GetContext().GetPlaying()
	switch in.GetName() {
	case "pause":
		return ctl("pause", 0, "paused", nil)
	case "resume":
		return ctl("resume", 0, "resumed", nil)
	case "stop":
		return ctl("stop", 0, "stopped", nil)
	case "mute":
		return ctl("mute", 0, "muted", nil)
	case "unmute":
		return ctl("unmute", 0, "unmuted", nil)
	case "fullscreen":
		return ctl("fullscreen", 0, "fullscreen", nil)
	case "seek_back", "seek_forward":
		secs, _ := s.number("seconds")
		key := "seeked_forward"
		if in.GetName() == "seek_back" {
			secs, key = -secs, "seeked_back"
		}
		return ctl("seek_by", secs, key, map[string]string{"duration": t.duration(math.Abs(secs))})
	case "seek_to":
		pos, _ := s.number("position")
		return ctl("seek", pos, "seeked_to", map[string]string{"position": clock(pos)})
	case "restart":
		return ctl("seek", 0, "restarted", nil)
	case "skip":
		return t.skip(s.keyword("segment"))
	case "next", "previous":
		return t.step(in.GetName(), device)
	case "speed":
		speed, ok := s.number("speed")
		if !ok {
			speed = playing.GetSpeed()
			if speed == 0 {
				speed = 1
			}
			switch s.keyword("change") {
			case "up":
				speed = min(speed+0.25, 3)
			case "down":
				speed = max(speed-0.25, 0.25)
			default:
				speed = 1
			}
		}
		return ctl("speed", speed, "speed", map[string]string{"speed": strconv.FormatFloat(speed, 'f', -1, 64)})
	case "volume":
		level, ok := s.number("level")
		if !ok {
			level = float64(playing.GetVolume())
			if !playing.HasVolume() {
				level = 50
			}
			if s.keyword("change") == "down" {
				level -= 10
			} else {
				level += 10
			}
		}
		level = min(max(math.Round(level), 0), 100)
		return ctl("volume", level, "volume", map[string]string{"level": strconv.Itoa(int(level))})
	case "subtitles_on":
		lang := s.language("language")
		if _, err := c.setStreams(streamsArgs{SubtitleLanguage: lang, SubtitlesOn: lang == ""}); err != nil {
			return outcome{}, t.named(err)
		}
		if lang == "" {
			return outcome{key: "subtitles_on"}, nil
		}
		return outcome{key: "subtitles_language", args: map[string]string{"language": t.languageName(lang)}}, nil
	case "subtitles_off":
		if _, err := c.setStreams(streamsArgs{SubtitlesOff: true}); err != nil {
			return outcome{}, err
		}
		return outcome{key: "subtitles_off"}, nil
	case "audio":
		lang := s.keyword("language")
		if lang == "" {
			lang = s.language("language")
		}
		if _, err := c.setStreams(streamsArgs{AudioLanguage: lang}); err != nil {
			return outcome{}, t.named(err)
		}
		return outcome{key: "audio", args: map[string]string{"language": t.languageName(lang)}}, nil
	case "subtitle_offset":
		secs, _ := s.number("seconds")
		dir := s.keyword("direction")
		off := secs
		if dir == "earlier" {
			off = -secs
		}
		if _, err := c.setStreams(streamsArgs{SubtitleOffset: &off}); err != nil {
			return outcome{}, err
		}
		return outcome{key: "subtitle_offset", args: map[string]string{
			"duration": t.duration(secs), "direction": t.s.reply(c.ctx, t.lang, dir, nil),
		}}, nil
	case "play":
		return t.play(s, device, "")
	case "continue":
		if s.text("title") != "" {
			return t.play(s, device, "continuing")
		}
		return t.continueWatching(device)
	case "shuffle":
		return t.shuffle(s, device)
	case "transfer":
		return t.transfer(device)
	case "search":
		q := s.text("query")
		if _, err := c.navigate(navigateArgs{Target: "search", Query: q}); err != nil {
			return outcome{}, err
		}
		return outcome{key: "searching", args: map[string]string{"query": q}}, nil
	case "open":
		items, err := t.titled(s.text("title"), titleKinds)
		if err != nil {
			return outcome{}, err
		}
		if len(items) > 1 {
			return t.show(items, "open"), nil
		}
		return t.act(items[0], "open")
	case "go_home":
		return t.navigate("home", "", "home", nil)
	case "go_back":
		return t.navigate("back", "", "back", nil)
	case "open_settings":
		return t.navigate("settings", "", "settings", nil)
	case "open_library":
		return t.openLibrary(s.text("library"))
	case "now_playing":
		np, err := c.nowPlaying(device)
		if err != nil {
			return outcome{}, err
		}
		return outcome{key: "now_playing", args: map[string]string{"title": title(np.item)}}, nil
	case "cast":
		return t.cast()
	case "time_left", "end_time":
		return t.remaining(in.GetName(), device)
	case "mark_played", "mark_unplayed", "favorite", "unfavorite":
		return t.userData(in.GetName(), s.text("title"))
	case "add_to_playlist":
		name := s.text("playlist")
		if name == "" {
			return outcome{key: "no_playlist", args: map[string]string{"playlist": ""}}, nil
		}
		id, err := t.subject()
		if err != nil {
			return outcome{}, err
		}
		res, err := c.addToPlaylist(playlistArgs{Playlist: name, ItemIDs: []string{id}})
		if err != nil {
			return outcome{}, err
		}
		return outcome{key: "added_to_playlist", args: map[string]string{"playlist": fmt.Sprint(res.(map[string]any)["playlist"])}}, nil
	case "request":
		return t.confirm(t.s.tool["request_title"], requestArgs{Query: s.text("title")})
	case "choose":
		n, _ := s.number("choice")
		return t.chooseNth(int(n))
	case "more":
		t.d.mu.Lock()
		more := t.d.offset+pageSize < len(t.d.results)
		if more {
			t.d.offset += pageSize
		}
		t.d.mu.Unlock()
		if !more {
			return outcome{key: "no_more"}, nil
		}
		o := t.page()
		o.text = t.s.reply(c.ctx, t.lang, "more", nil) + " " + t.s.reply(c.ctx, t.lang, o.key, o.args)
		return o, nil
	case "cancel":
		t.d.mu.Lock()
		t.d.pending, t.d.results = nil, nil
		t.d.mu.Unlock()
		return outcome{key: "cancelled"}, nil
	case "help":
		text := t.s.reply(c.ctx, t.lang, "help", nil)
		if ex, err := t.s.cfg.Grammar.Examples(c.ctx, intentv1.ExamplesRequest_builder{Language: &t.lang}.Build()); err == nil {
			text += "\n" + strings.Join(ex.GetExamples(), "\n")
		}
		return outcome{text: text}, nil
	}
	return outcome{key: "not_understood", notUnderstood: true}, nil
}

// named makes a language's failure name the language in the answer's.
func (t *turn) named(err error) error {
	if ae := (*answerError)(nil); errors.As(err, &ae) && ae.args["language"] != "" {
		ae.args["language"] = t.languageName(ae.args["language"])
	}
	return err
}

// languageName names a language in the answer's language.
func (t *turn) languageName(code string) string {
	if code == "original" {
		return t.s.reply(t.call.ctx, t.lang, "original", nil)
	}
	tag, err := language.Parse(code)
	if err != nil {
		return code
	}
	in, err := language.Parse(t.lang)
	if err != nil {
		in = language.English
	}
	if n := display.Tags(in).Name(tag); n != "" {
		return n
	}
	return code
}

// duration says a number of seconds with the grammar's units.
func (t *turn) duration(secs float64) string {
	d := time.Duration(math.Round(secs)) * time.Second
	h, m, sec := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	var parts []string
	for _, p := range []struct {
		n   int
		key string
	}{{h, "hours"}, {m, "minutes"}, {sec, "seconds"}} {
		if p.n > 0 {
			parts = append(parts, t.s.reply(t.call.ctx, t.lang, p.key, map[string]string{"n": strconv.Itoa(p.n)}))
		}
	}
	if len(parts) == 0 {
		return t.s.reply(t.call.ctx, t.lang, "seconds", map[string]string{"n": "0"})
	}
	return strings.Join(parts, " ")
}

// clock writes a position as h:mm:ss or m:ss.
func clock(secs float64) string {
	d := time.Duration(math.Round(secs)) * time.Second
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (t *turn) navigate(target, id, key string, args map[string]string) (outcome, error) {
	if _, err := t.call.navigate(navigateArgs{Target: target, ID: id}); err != nil {
		return outcome{}, err
	}
	return outcome{key: key, args: args}, nil
}

// titled resolves a title, failing with no_title when nothing has it.
func (t *turn) titled(text string, ks []libraryv1.ItemKind) ([]*libraryv1.Item, error) {
	items, err := t.call.resolveTitle(text, ks)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, failure("no_title", map[string]string{"title": text}, "no title %q", text)
	}
	return items, nil
}

// subject is the item "it" means: what plays, else the item shown or
// selected, else the conversation's last.
func (t *turn) subject() (string, error) {
	ctx := t.req.GetContext()
	switch {
	case ctx.GetPlaying().GetItemId() != "":
		return ctx.GetPlaying().GetItemId(), nil
	case ctx.GetItemId() != "":
		return ctx.GetItemId(), nil
	case len(ctx.GetSelectedItemIds()) > 0:
		return ctx.GetSelectedItemIds()[0], nil
	}
	t.d.mu.Lock()
	defer t.d.mu.Unlock()
	if t.d.lastItem != "" {
		return t.d.lastItem, nil
	}
	return "", failure("nothing_playing", nil, "nothing is playing or shown")
}

func (t *turn) play(s slots, device, key string) (outcome, error) {
	items, err := t.titled(s.text("title"), titleKinds)
	if err != nil {
		return outcome{}, err
	}
	if len(items) > 1 {
		return t.show(items, "play"), nil
	}
	it := items[0]
	season, hasSeason := s.number("season")
	episode, hasEpisode := s.number("episode")
	if it.GetKind() == libraryv1.ItemKind_ITEM_KIND_SERIES && (hasSeason || hasEpisode) {
		if !hasSeason {
			season = 1
		}
		ep, err := t.call.episode(it, int32(season), int32(episode))
		if err != nil {
			return outcome{}, err
		}
		if ep == nil {
			return outcome{}, failure("no_title", map[string]string{"title": s.text("title")}, "no such episode")
		}
		it = ep
	}
	res, err := t.call.play(playArgs{ItemIDs: []string{it.GetId()}, Device: device})
	if err != nil {
		return outcome{}, err
	}
	r := res.(playResult)
	switch {
	case key != "":
		return outcome{key: key, args: map[string]string{"title": r.Playing}}, nil
	case r.Device != "":
		return outcome{key: "playing_on", args: map[string]string{"title": r.Playing, "device": r.Device}}, nil
	}
	return outcome{key: "playing", args: map[string]string{"title": r.Playing}}, nil
}

// continueWatching plays what the user last watched and did not finish,
// else the next episode of what they watch.
func (t *turn) continueWatching(device string) (outcome, error) {
	res, err := t.call.listResume(listArgs{Limit: 1})
	if err != nil {
		return outcome{}, err
	}
	items := res.(itemsResult).items
	if len(items) == 0 {
		next, err := t.call.listNextUp(listArgs{Limit: 1})
		if err != nil {
			return outcome{}, err
		}
		items = next.(itemsResult).items
	}
	if len(items) == 0 {
		return outcome{key: "nothing_to_continue"}, nil
	}
	played, err := t.call.play(playArgs{ItemIDs: []string{items[0].GetId()}, Device: device})
	if err != nil {
		return outcome{}, err
	}
	return outcome{key: "continuing", args: map[string]string{"title": played.(playResult).Playing}}, nil
}

func (t *turn) shuffle(s slots, device string) (outcome, error) {
	query := s.text("query")
	var ids []string
	switch {
	case s.keyword("kind") == "favorites":
		res, err := t.call.searchLibrary(searchArgs{
			Favorite: new(true), Sort: "random", Limit: 50,
			Kinds: []string{"movie", "episode", "track", "music_video", "video"},
		})
		if err != nil {
			return outcome{}, err
		}
		for _, it := range res.(itemsResult).items {
			ids = append(ids, it.GetId())
		}
		if query == "" {
			query = s.text("kind")
		}
	case query != "":
		items, err := t.titled(query, titleKinds)
		if err != nil {
			return outcome{}, err
		}
		if items[0].GetKind() == libraryv1.ItemKind_ITEM_KIND_SERIES {
			eps, err := t.call.children(items[0].GetId(), []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, 1000)
			if err != nil {
				return outcome{}, err
			}
			for _, e := range eps {
				ids = append(ids, e.GetId())
			}
		} else {
			ids = []string{items[0].GetId()}
		}
	default:
		res, err := t.call.searchLibrary(searchArgs{Kinds: []string{"movie"}, Sort: "random", Limit: 1, Played: new(false)})
		if err != nil {
			return outcome{}, err
		}
		for _, it := range res.(itemsResult).items {
			ids = append(ids, it.GetId())
		}
	}
	if len(ids) == 0 {
		return outcome{key: "no_title", args: map[string]string{"title": query}}, nil
	}
	if _, err := t.call.play(playArgs{ItemIDs: ids, Shuffle: true, Start: "beginning", Device: device}); err != nil {
		return outcome{}, err
	}
	return outcome{key: "shuffling", args: map[string]string{"query": query}}, nil
}

// transfer moves what plays here to another device.
func (t *turn) transfer(device string) (outcome, error) {
	p := t.req.GetContext().GetPlaying()
	if p.GetItemId() == "" {
		return outcome{key: "nothing_playing"}, nil
	}
	if t.call.here(device) {
		return outcome{key: "no_device", args: map[string]string{"device": device}}, nil
	}
	pos := p.GetPosition().AsDuration().Seconds()
	res, err := t.call.play(playArgs{ItemIDs: []string{p.GetItemId()}, Position: &pos, Device: device})
	if err != nil {
		return outcome{}, err
	}
	if _, err := t.call.control(controlArgs{Command: "stop"}); err != nil {
		return outcome{}, err
	}
	return outcome{key: "transferred", args: map[string]string{"device": res.(playResult).Device}}, nil
}

// skip skips a media segment of what plays here.
func (t *turn) skip(segment string) (outcome, error) {
	kind := map[string]string{"credits": "outro", "": "intro"}[segment]
	if kind == "" {
		kind = segment
	}
	if id := t.req.GetContext().GetPlaying().GetItemId(); id != "" {
		resp, err := t.s.cfg.Services.Items.GetItem(t.call.ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
		if err != nil {
			return outcome{}, err
		}
		found := false
		for _, seg := range resp.GetSegments() {
			if strings.EqualFold(strings.TrimPrefix(seg.GetKind().String(), "SEGMENT_KIND_"), kind) {
				found = true
			}
		}
		if !found {
			return outcome{key: "no_segment"}, nil
		}
	}
	if _, err := t.call.control(controlArgs{Command: "skip", Segment: kind}); err != nil {
		return outcome{}, err
	}
	return outcome{key: "skipped"}, nil
}

// step plays the next or previous episode, naming it.
func (t *turn) step(dir, device string) (outcome, error) {
	np, err := t.call.nowPlaying(device)
	if err != nil {
		return outcome{}, err
	}
	key := dir
	args := map[string]string{"title": ""}
	if it := np.item; it.GetKind() == libraryv1.ItemKind_ITEM_KIND_EPISODE && it.GetSeriesId() != "" {
		eps, err := t.call.children(it.GetSeriesId(), []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, 1000)
		if err != nil {
			return outcome{}, err
		}
		i := -1
		for j, e := range eps {
			if e.GetId() == it.GetId() {
				i = j
			}
		}
		delta := 1
		if dir == "previous" {
			delta = -1
		}
		if j := i + delta; i < 0 || j < 0 || j >= len(eps) {
			return outcome{key: "no_" + dir}, nil
		} else {
			args["title"] = title(eps[j])
		}
	}
	if _, err := t.call.control(controlArgs{Command: dir, Device: device}); err != nil {
		return outcome{}, err
	}
	if args["title"] == "" {
		return outcome{key: map[string]string{"next": "skipped", "previous": "restarted"}[dir]}, nil
	}
	return outcome{key: key, args: args}, nil
}

func (t *turn) openLibrary(name string) (outcome, error) {
	resp, err := t.s.cfg.Services.Libraries.ListLibraries(t.call.ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil {
		return outcome{}, err
	}
	want := normalize(name)
	for _, l := range resp.GetLibraries() {
		if n := normalize(l.GetName()); n == want || strings.Contains(n, want) || (hasHan(name) && pinyinKey(l.GetName()) == pinyinKey(name)) {
			return t.navigate("library", l.GetId(), "library", map[string]string{"library": l.GetName()})
		}
	}
	return outcome{key: "no_title", args: map[string]string{"title": name}}, nil
}

func (t *turn) cast() (outcome, error) {
	id, err := t.subject()
	if err != nil {
		return outcome{}, err
	}
	resp, err := t.s.cfg.Services.Items.GetItem(t.call.ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
	if err != nil {
		return outcome{}, err
	}
	var names []string
	for _, cr := range resp.GetCredits() {
		if k := cr.GetKind(); (k == libraryv1.CreditKind_CREDIT_KIND_ACTOR || k == libraryv1.CreditKind_CREDIT_KIND_GUEST_STAR) && len(names) < 3 {
			names = append(names, cr.GetPerson().GetName())
		}
	}
	t.call.item(id)
	if len(names) == 0 {
		return outcome{key: "now_playing", args: map[string]string{"title": title(resp.GetItem())}}, nil
	}
	return outcome{key: "cast", args: map[string]string{"title": title(resp.GetItem()), "cast": strings.Join(names, ", ")}}, nil
}

// remaining answers how long is left, or when it ends.
func (t *turn) remaining(what, device string) (outcome, error) {
	np, err := t.call.nowPlaying(device)
	if err != nil {
		return outcome{}, err
	}
	left := np.item.GetRuntime().AsDuration().Seconds() - np.Position
	if left < 0 {
		left = 0
	}
	if what == "time_left" {
		return outcome{key: "time_left", args: map[string]string{"remaining": t.duration(left)}}, nil
	}
	speed := np.Speed
	if speed <= 0 {
		speed = 1
	}
	end := t.s.now().Add(time.Duration(left / speed * float64(time.Second)))
	if loc, err := time.LoadLocation(t.req.GetContext().GetTimeZone()); err == nil && t.req.GetContext().GetTimeZone() != "" {
		end = end.In(loc)
	}
	return outcome{key: "end_time", args: map[string]string{"time": end.Format("15:04")}}, nil
}

func (t *turn) userData(what, titleText string) (outcome, error) {
	var id string
	if titleText != "" {
		items, err := t.titled(titleText, append(titleKinds, libraryv1.ItemKind_ITEM_KIND_EPISODE))
		if err != nil {
			return outcome{}, err
		}
		if len(items) > 1 {
			return t.show(items, "open"), nil
		}
		id = items[0].GetId()
	} else {
		var err error
		if id, err = t.subject(); err != nil {
			return outcome{}, err
		}
	}
	a := userDataArgs{ItemID: id}
	key := map[string]string{"mark_played": "marked_played", "mark_unplayed": "marked_unplayed", "favorite": "favorited", "unfavorite": "unfavorited"}[what]
	switch what {
	case "mark_played", "mark_unplayed":
		a.Played = new(what == "mark_played")
	default:
		a.Favorite = new(what == "favorite")
	}
	if _, err := t.call.setUserData(a); err != nil {
		return outcome{}, err
	}
	return outcome{key: key}, nil
}

// chooseNth acts on the n-th result shown, -1 the last.
func (t *turn) chooseNth(n int) (outcome, error) {
	t.d.mu.Lock()
	page := t.d.results[t.d.offset:min(len(t.d.results), t.d.offset+pageSize)]
	purpose := t.d.purpose
	t.d.mu.Unlock()
	if n < 0 {
		n = len(page)
	}
	if n < 1 || n > len(page) {
		return outcome{key: "not_understood", notUnderstood: true}, nil
	}
	return t.act(page[n-1], purpose)
}
