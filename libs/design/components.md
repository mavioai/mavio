# Components

> English | [简体中文](components.zh-CN.md)

The core components of the clients ([Clients §6.4](../../docs/clients.md#64-components)). Each client implements them natively with the tokens of `gen/`; this document is what the implementations share. Names in `code` are tokens: `color.*` of the theme, `layout.*` of the form factor, `typography.*`, `spacing.*`, `radius.*`, `duration.*` and `easing.*`.

## States

Every interactive component has these states; television adds focused to every card.

| State | Appearance |
| :--- | :--- |
| Default | As specified below |
| Focused (television, keyboard) | Scales by `layout.focusScale` with `elevation.level2`, over `duration.fast` with `easing.standard`; a 2 pt `color.focus` ring on keyboard focus elsewhere |
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

* **Anatomy**: the title (`typography.title3`) with a "more" action on desktops and touch screens; the cards below it, `layout.gutter` apart, the first aligned with `layout.margin`.
* **Behavior**: scrolls horizontally by touch, wheel and trackpad; on television focus moves card by card and the row scrolls to keep the focused card in view; on desktops arrows appear on hover at both ends.
* **Empty**: a shelf with no items is hidden.

## Details Header

The top of an item's details screen.

* **Anatomy**: the backdrop filling the width, fading into `color.background`; over its lower part the logo or title (`typography.display`), a line of facts (`typography.callout`, `color.textSecondary`: year, rating, runtime, resolution), the primary action (Play or Resume, `color.button` with `color.onButton`), and secondary actions (favorite, mark played, more).
* **Layout**: phones stack the poster above the text; tablets, desktops and television place the text at the left of the backdrop.
* **Overview**: `typography.body`, three lines with a "more" action on phones.

## Episode Row

An episode in a season's list.

* **Anatomy**: a thumbnail card's artwork at the left (width `layout.thumbnail` / 2 on phones); the number and title (`typography.headline`), the runtime and air date (`typography.footnote`, `color.textSecondary`) and two lines of overview (`typography.callout`).
* **Overlays**: as the thumbnail card.

## Person Card

A cast or crew member.

* **Anatomy**: a circular photo (`radius.full`), the name (`typography.callout`) and the role (`typography.footnote`, `color.textSecondary`), each on at most two lines.
* **Without a photo**: the initials on `color.surfaceRaised`.

## Track Picker

The audio and subtitle pickers of the player and of the details screen.

* **Anatomy**: a list of tracks, each with its language name and its details (`typography.footnote`, `color.textSecondary`: codec, channels, forced, hearing impaired); the selected one marked with a check. Subtitles start with `player.subtitlesOff`.
* **Presentation**: a menu on Apple platforms and desktops, a bottom sheet on Android phones, a side panel on television.

## Status View

The empty, error, offline and loading states of a screen or list.

* **Anatomy**: an icon (48 pt, `color.textTertiary`: `error`, `offline`), a title (`typography.title3`: `state.empty`, `state.error`, `state.offline`), an optional message (`typography.body`, `color.textSecondary`) and an optional action (`action.retry`).
* **Loading**: placeholders of the content (see States) rather than a spinner, except for the first screen, which shows the app's logo.
* **Offline**: downloaded items stay listed under the view.
