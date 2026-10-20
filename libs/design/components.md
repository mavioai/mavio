# Components

> English | [简体中文](components.zh-CN.md)

The core components of the clients ([Clients §6.4](../../docs/clients.md#64-components)). Each client implements them natively with the tokens of `gen/`; this document is what the implementations share. Names in `code` are tokens: `color.*` of the theme, `layout.*` of the form factor, `typography.*`, `spacing.*`, `radius.*`, `duration.*` and `easing.*`.

## States

Every interactive component has these states; television adds focused to every card.

| State | Appearance |
| :--- | :--- |
| Default | As specified below |
| Hovered (pointer) | A card's artwork gains a 2 pt `color.accent` border, 1 pt apart from it; a row, a list's entry or a plain button lays `color.fill` over its background, darkening it in the light theme and lightening it in the dark |
| Focused (television, keyboard) | Scales by `layout.focusScale` with a shadow of `color.tvFocusShadow` (offset 12 pt, blur 32 pt) and a 3 pt `color.accent` border on the artwork, 1.5 pt apart from it, over `duration.fast` with `easing.standard`; a 2 pt `color.focus` ring on keyboard focus elsewhere |
| Pressed | Content dimmed to 80% opacity |
| Loading | A `color.surfaceRaised` placeholder of the component's final size, so that nothing moves when content arrives |
| Disabled | Content in `color.textDisabled` |
| No artwork | The placeholder stays, with a wash of `color.fill`, a quiet gray, from its top left, gone by 70% of the way, and the icon of what it shows (a movie, a series, music…) in `color.textTertiary` at 60%, a quarter of its width and at most 56 pt; never the title written on it |

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
* **Actions**: the primary action, white with black content, plays what is next: Resume or Play for a movie or episode; for a series or season the episode to watch next with its number, "Resume S1 E2". Beside it, as circles, Play from Beginning when there is progress, the trailer when the item has one among its extras, else its trailer on the web, opened in a browser, favorite and played, then the version menu when the title has several versions or is on other servers. Under them a status line (`typography.footnote`): the progress bar, the episode's name, the time left and when it would end.
* **Next episode**: of a series' episodes, ignoring specials unless there are only specials, the one played last if it is unfinished, else the first unplayed one after it, else the first unplayed one, else the first.
* **Version menu**: a pill "Version 2160p" opening upward a menu of the versions, each named by what its file name adds to its folder's ("2160p") with its resolution, range, surround, container and size, the chosen one checked; then the same title on the user's other servers.
* **Layout**: tablets, desktops and television place the text at the left of the artwork, which fills 70% of the window's height (520–700 pt) on desktops, a rectangle reaching the edges of the content beside the sidebar, and 86% of the screen on television, where focus starts on the primary action and the tabs are hidden. Phones show the artwork as a square as wide as the screen, reaching up under the clear header to the top, which adds the header's height to it, with the logo or title, facts and badges, and put the actions and overview below it on the page, the primary action across the width on a row of its own above the circles.

## Details Screen

Below the header, in order:

* **Episodes**: a series has its seasons as pills, starting with the season of the next episode, and that season's episodes in a row of episode cards, with See All opening the season and, while the series has seasons left to request, Request More choosing them as on its discovered title; on television focus on a pill chooses it. A season lists its episodes as episode rows. An episode shows its season's other episodes in a row, scrolled to it.
* **Cast and crew**: a shelf of person cards, the actors in billing order, each once as the character they play the longest, then the creators, directors and writers, with See All opening the Credits Screen.
* **Trailers and extras**: a shelf of thumbnail cards of the item's extras other than theme songs and videos, each with a play mark, its name and its kind and runtime, played on selection; an extra without a thumbnail wears the item's backdrop.
* **Collection**: a movie in a collection has a banner of the collection's backdrop, else the movie's, darkened toward the left, with "Collection" over its name and View Collection at the right (radius 14 pt).
* **Similar**: a shelf of posters of the server's items like it.
* **Recommendations**: for a movie or series with a TMDB ID, the discovery providers' recommendations as a shelf of title cards, as on a discovered title.
* **Details**: at the left the tagline, the whole overview, the six leading crew with View Full Crew opening the Credits Screen with the crew alone, as on a discovered title, the keywords (the item's tags) as outlined pills and the links to the external sites — for a movie or series the logos of its discovered title, else as pills; at the right the ratings — for a movie or series those of its discovered title, Rotten Tomatoes', IMDb's and TMDB's, each opening its page, else, and without them, the audience's out of 10 with a star and the critics' in percent, each large on `color.fill` — then the facts (release or air date, a movie's theatrical, digital and physical releases on one row, each with its icon, last and next air date, status, runtime, official rating, genres, studio or network, the first five with Show More, country, with their flags when the library keeps the countries the metadata providers tell, original language, original title, budget, revenue, date added) and the services streaming it, each a pill of its logo and name and below them the chosen version's video, audio, subtitles, file and size, each a caption (`typography.caption`, `color.textSecondary`) over its value. Genres, keywords and studios open the library's items of that value. What the library keeps of the item is shown first, what the metadata providers tell of its discovered title filling what it lacks. For administrators the header shows how far a movie's or series' downloads have come, and its acquisition lists its requests, to approve or decline, before its downloads.

## Credits Screen

An item's or a discovered title's whole cast and crew, opened from its details: a link back to it over the title, then sections of person cards in grids — the actors in billing order, the guest stars, then the crew by job (creators, directors, writers, authors, producers, music, conductors, lyricists, artists, narrators, other crew), each card with the character played or the job's own title when known. People of the server open their page, others their discovered page. An item's or a discovered title's View Full Crew opens it with the crew alone, titled Full Crew, as Seerr's crew page.

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
* **Without a photo**: the No artwork placeholder of the cards (States), with the icon `profile`.

## Avatar

A user's or an account's picture in menus, lists of users and Homes.

* **Anatomy**: a circle (`radius.full`) holding the first character of the name, upper-cased and semibold, 0.42 of the circle's size.
* **Color**: the same for everyone and quiet against the background: the circle is `color.fillSelected`, the initial `color.textSecondary`, in each theme; the initial tells people apart.

## Track Picker

The audio and subtitle pickers of the player and of the details screen.

* **Anatomy**: a list of tracks, each with its language name and its details (`typography.footnote`, `color.textSecondary`: codec, channels, forced, hearing impaired); the selected one marked with a check. Subtitles start with `player.subtitlesOff`.
* **Presentation**: a menu on Apple platforms and desktops, a bottom sheet on Android phones, a side panel on television.

## Status View

The empty, error, offline and loading states of a screen or list.

* **Anatomy**: an icon (48 pt, `color.textTertiary`: `error`, `offline`), a title (`typography.title3`: `state.empty`, `state.error`, `state.offline`), an optional message (`typography.body`, `color.textSecondary`) and an optional action (`action.retry`).
* **Loading**: placeholders of the content (see States) where its shape is known, as a shelf's cards; else, as a page on its way, the activity indicator, centered in the space the page is given: eight spokes in a circle of 28 pt, `color.textTertiary`, fading from 20% to full opacity around it and turning a spoke a step, once in 0.9 s, still for reduced motion. It appears 0.4 s after loading starts, so that what comes quickly shows nothing first. No server's name or address stands for content still loading. The first screen shows the app's logo.
* **Offline**: downloaded items stay listed under the view.

## Assistant

The voice assistant ([Assistant §9](../../docs/assistant.md#9-clients)): an orb floating on its own over the page, and cards beside it of what the user said, what the assistant answers and what it does. There is no dialog, header, close button or text field: the assistant is spoken to. [`prototypes/assistant.html`](prototypes/assistant.html) draws every state of the orb and every card in both themes, on a desktop and a phone.

* **Layout**: on desktops, tablets and television the orb floats at the top right, 84 pt, and the cards stack below it, 380 pt wide and `spacing.sm` apart, the newest at the bottom; on phones the orb floats at the bottom center and the cards stack above it, as wide as the screen less `spacing.sm` at each side, the newest at the top. A status pill (`typography.footnote` on the cards' material, `radius.full`) beside the orb says what it awaits, "Loading…", "Listening…" or how to talk, and is hidden otherwise. The page behind stays as it is and usable once the assistant is dismissed.
* **Orb**: a sphere of glass with colored light flowing inside it, with no border or background: three blurred blobs, magenta, cyan and blue, each a radial gradient orbiting the center at its own speed, clipped to the sphere (a ring while thinking) over a body of the blue. Over them the glass: darker toward the lower edge, light caught along the rim, and a soft oval reflection at the top left. In the dark theme the body is a deep blue, the colors add up and glow white at the core, and a faint glow of them spreads around the sphere; in the light theme the colors are deeper (magenta `#EB28A0`, cyan `#00AAEB`, blue `#5A46F5`) and laid over each other on the blue, with a small shadow of the blue, so that the core keeps its color and the edge does not smudge into the page. Four values drive it, eased toward each state's in about 0.15 s: size, spread (how far the blobs orbit apart), speed and brightness; a level of the audio (the microphone's when the user speaks, the playback's when the assistant does) swells it and spreads the blobs. Reduced motion keeps the orb still at each state's size and brightness.
* **States**: the orb shows the voice session's state, from its events ([Assistant §8.6](../../docs/assistant.md#86-protocol)):

| State | When | The orb |
| :--- | :--- | :--- |
| Opening | From the press until the microphone's first audio | Small and dim, breathing slowly; the pill says loading |
| Listening | Audio comes and the user is silent: before and after `ready`, and after an answer | Full size, the colors flowing slowly, breathing lightly; the pill says listening |
| User speaking | `user_started` → `user_stopped`, `transcript` growing meanwhile | Swells and shrinks with the microphone's level, the colors spreading out |
| Pausing | The user falls silent but the turn is not over (no `user_stopped` yet) | Draws in and turns slowly |
| Thinking | `user_stopped` or `run` → `bot_started`, with `tool` meanwhile | Gathers into a ring spinning fast |
| Speaking | `bot_started` → `bot_stopped` | Rises and falls with the playback's level |
| Interrupted | `interrupt` | Contracts with a white flash, then turns to User speaking |
| Not listening | Push to talk released, or the session ended while cards remain | Full size and color, flowing very slowly, not breathing; the pill says how to talk |
| Error | `error`, or the session cannot be reached | Shakes once and flushes red, then turns to Not listening |
| Ended | The session closes or idles out with nothing to choose or confirm | Shrinks and fades out with the cards |

* **Cards**: of the cards' material — `color.surfaceOverlay` at 72% in the dark theme and white at 74% in the light, over a blur of the page, with a 1 pt `color.border`, `radius.xl` and `elevation.level3` — rising into place over `duration.slow` with `easing.emphasized`:
  * **Talk**: what the user said, then the answer, both `typography.body`. What the user said is `color.textSecondary`, growing with `transcript`, and once final ends with a chevron "›" in `color.textTertiary`; the answer's words start in `color.textTertiary` and turn `color.textPrimary` as they are spoken, paced by the playback (all at once when nothing is spoken).
  * **Tools**: a row for each `tool`: a 26 pt `radius.sm` tile of the tool's icon in `color.accent` on the accent at 16%, its summary (`typography.footnote`, `color.textSecondary`), and an activity indicator while it runs, then a check in `color.success` or a cross in `color.danger`.
  * **Results**: the items to choose from, numbered so that the user can say "the second", each a row of its number (`color.textTertiary`), poster and title with its year; choosing one by voice, click or tap acts on it.
  * **Confirmation**: the question, and Cancel and Confirm (`color.accent`) side by side, answerable by voice as well.
  * **Error**: the message in `color.danger`'s text.
  The cards of a turn replace the previous turn's; Results and Confirmation stay until answered.
* **Dismissal**: on its own, a few seconds after the session ends or after an answer with nothing to choose or confirm (the Ended state), held while the pointer is over a card or a card is touched; by a click or tap anywhere outside the orb and the cards; by Escape, the remote's Back or Android's back. Each ends the session, and the orb and cards fade out together over `duration.slow`.
