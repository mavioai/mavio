# Components

> English | [简体中文](components.zh-CN.md)

The core components of the clients ([Clients §6.4](../../docs/clients.md#64-components)). Each client implements them natively with the tokens of `gen/`; this document is what the implementations share. Names in `code` are tokens: `color.*` of the theme, `layout.*` of the form factor, `typography.*`, `spacing.*`, `radius.*`, `duration.*` and `easing.*`.

## States

Every interactive component has these states; television adds focused to every card.

| State | Appearance |
| :--- | :--- |
| Default | As specified below |
| Hovered (pointer) | A card's artwork gains a 2 pt `color.accent` border, 1 pt apart from it |
| Focused (television, keyboard) | Scales by `layout.focusScale` with a shadow of `color.tvFocusShadow` (offset 12 pt, blur 32 pt) and a 3 pt `color.accent` border on the artwork, 1.5 pt apart from it, over `duration.fast` with `easing.standard`; a 2 pt `color.focus` ring on keyboard focus elsewhere |
| Pressed | Content dimmed to 80% opacity |
| Loading | A `color.surfaceRaised` placeholder of the component's final size, so that nothing moves when content arrives |
| Disabled | Content in `color.textDisabled` |

## Poster Card

A 2:3 card of a movie, series, season or collection.

* **Anatomy**: artwork with `radius.md`; below it the title (`typography.headline`, one line) and a subtitle (`typography.footnote`, `color.textSecondary`, one line: year, or the number of items).
* **Overlays**: a watched progress bar (3 pt, `color.progress` over `color.scrim`) along the bottom edge of the artwork; a played check (icon `played`, Fill) in the top right corner; the number of unplayed episodes in a `color.accent` badge in its place for series and seasons.
* **Size**: width `layout.poster`; in grids the width is the space divided by the columns that fit at least `layout.poster`.
* **Without artwork**: the title in `typography.title3` centered on `color.surfaceRaised`.

## Thumbnail Card

A 16:9 card of an episode, a video or a chapter.

* **Anatomy**: artwork with `radius.md`; below it the title (`typography.headline`) and, for episodes, the series and episode numbers (`typography.footnote`, `color.textSecondary`, string `item.episodeNumber`).
* **Overlays**: as the poster card, plus the time left (`item.minutesLeft`) of an unfinished item in the bottom left corner.
* **Size**: width `layout.thumbnail`.

## Shelf

A titled horizontal row of cards (Continue Watching, Next Up, Latest in a library).

* **Anatomy**: the title (`typography.title3`) with a "more" action on desktops and touch screens; the cards below it, one and a half `layout.gutter` apart (`layout.gutter` on television), the first aligned with `layout.margin`.
* **Behavior**: scrolls horizontally by touch, wheel and trackpad; on television focus moves card by card and the row scrolls to keep the focused card in view; where a mouse points, two arrows at the end of the title scroll it back and forward by most of its width, each dimmed at its end and both hidden when the cards fit; television has none.
* **Empty**: a shelf with no items is hidden.

## Details Header

The top of an item's details screen.

* **Anatomy**: the backdrop, else an episode's still, else the poster blurred behind the poster itself; over its lower left, darkened toward it, the series an episode or season belongs to (its logo, linking to it), an episode's number, the logo or title (`typography.display`), the facts, the ratings and badges, the tagline (`typography.headline`), the overview and the actions.
* **Facts**: one line (`typography.callout`) of the year, or a series' years ("2023–") and number of seasons, a season's number of episodes, an episode's air date, the runtime and up to three genres, each separator kept with the fact before it so that no line starts with one. Below it the community rating with a star, the critics' score, and badges outlined in white: the official rating, the chosen version's resolution, dynamic range and surround, and CC when it has subtitles.
* **Overview**: `typography.body`, three lines (two on television) with More when it is longer; More opens it whole with the tagline in a dialog, or expands it in place on phones.
* **Actions**: the primary action, white with black content, plays what is next: Resume or Play for a movie or episode; for a series or season the episode to watch next with its number, "Resume S1 E2". Beside it, as circles, Play from Beginning when there is progress, the trailer when the item has one among its extras, favorite and played, then the version menu when the title has several versions or is on other servers. Under them a status line (`typography.footnote`): the progress bar, the episode's name, the time left and when it would end.
* **Next episode**: of a series' episodes, ignoring specials unless there are only specials, the one played last if it is unfinished, else the first unplayed one after it, else the first unplayed one, else the first.
* **Version menu**: a pill "Version 2160p" opening upward a menu of the versions, each named by what its file name adds to its folder's ("2160p") with its resolution, range, surround, container and size, the chosen one checked; then the same title on the user's other servers.
* **Layout**: tablets, desktops and television place the text at the left of the artwork, which fills 70% of the window's height (520–700 pt) on desktops, inset with corners of 18 pt, and 86% of the screen on television, where focus starts on the primary action and the tabs are hidden. Phones show the artwork at 64% of the height with the logo or title, facts and badges, and put the actions and overview below it on the page.

## Details Screen

Below the header, in order:

* **Episodes**: a series has its seasons as pills, starting with the season of the next episode, and that season's episodes in a row of episode cards, with See All opening the season; on television focus on a pill chooses it. A season lists its episodes as episode rows. An episode shows its season's other episodes in a row, scrolled to it.
* **Cast and crew**: a shelf of person cards, the actors in billing order, then the creators, directors and writers, with See All opening the Credits Screen.
* **Trailers and extras**: a shelf of thumbnail cards of the item's extras other than theme songs and videos, each with a play mark, its name and its kind and runtime, played on selection; an extra without a thumbnail wears the item's backdrop.
* **Collection**: a movie in a collection has a banner of the collection's backdrop, else the movie's, darkened toward the left, with "Collection" over its name and View Collection at the right (radius 14 pt).
* **Similar**: a shelf of posters of the server's items like it.
* **Recommendations**: for a movie or series with a TMDB ID, the discovery providers' recommendations as a shelf of title cards, as on a discovered title.
* **Details**: at the left the tagline, the whole overview, the keywords (the item's tags) as outlined pills and links to the external sites; at the right the ratings — the audience's out of 10 with a star and the critics' in percent, each large on `color.fill` — then the facts (release or air date, status, runtime, official rating, genres, creators, directors, writers, producers, music, studio, country, original title, date added) and below them the chosen version's video, audio, subtitles, file and size, each a caption (`typography.caption`, `color.textSecondary`) over its value. Genres, keywords and studios open the library's items of that value.

## Credits Screen

An item's whole cast and crew, opened from its details: a link back to the item over the title, then sections of person cards in grids — the actors in billing order, the guest stars, then the crew by job (creators, directors, writers, authors, producers, music, conductors, lyricists, artists, narrators, other crew), each card with the character played or the job's own title when known.

## Episode Card

An episode in a series' row.

* **Anatomy**: a thumbnail card's artwork, `layout.thumbnail` wide; below it the number ("S1 E2", `color.accentText` for the next episode) with the runtime and air date (`typography.footnote`, `color.textSecondary`), the title (`typography.headline`) and three lines of overview (`typography.footnote`).
* **Overlays**: as the thumbnail card.

## Episode Row

An episode in a season's list.

* **Anatomy**: a thumbnail card's artwork at the left, a third of the row at most (140 pt to `layout.thumbnail`); the number (`color.accentText` for the next episode), the title (`typography.headline`), the runtime and air date (`typography.footnote`, `color.textSecondary`) and three lines of overview (`typography.callout`).
* **Overlays**: as the thumbnail card.

## Person Card

A cast or crew member.

* **Anatomy**: a circular photo (`radius.full`), the name (`typography.callout`) and the role (`typography.footnote`, `color.textSecondary`), each on at most two lines.
* **Without a photo**: the initials on `color.fill`.

## Avatar

A user's or an account's picture in menus, lists of users and Homes.

* **Anatomy**: a circle (`radius.full`) holding the first character of the name, upper-cased, in white and semibold, 0.42 of the circle's size.
* **Color**: drawn from the name, so that a person has the same color everywhere and people side by side differ: the hue is 0, and for each character of the name, in code points, the hue times 31 plus the code point, modulo 360; the fill is that hue at 42% saturation and 44% lightness, on which white text stays legible.

## Track Picker

The audio and subtitle pickers of the player and of the details screen.

* **Anatomy**: a list of tracks, each with its language name and its details (`typography.footnote`, `color.textSecondary`: codec, channels, forced, hearing impaired); the selected one marked with a check. Subtitles start with `player.subtitlesOff`.
* **Presentation**: a menu on Apple platforms and desktops, a bottom sheet on Android phones, a side panel on television.

## Status View

The empty, error, offline and loading states of a screen or list.

* **Anatomy**: an icon (48 pt, `color.textTertiary`: `error`, `offline`), a title (`typography.title3`: `state.empty`, `state.error`, `state.offline`), an optional message (`typography.body`, `color.textSecondary`) and an optional action (`action.retry`).
* **Loading**: placeholders of the content (see States) rather than a spinner, except for the first screen, which shows the app's logo.
* **Offline**: downloaded items stay listed under the view.
