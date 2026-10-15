# Mavio on Television

> English | [简体中文](tv.zh-CN.md)

> The television layout of Mavio's clients: the Apple TV app, the Android TV and Google TV app, and the television mode of the web UI, the Mac app and the desktop app. It is watched from across a room and driven by a remote alone, so it has larger type and margins, focus in place of a pointer, and no hover. This document specifies it once; each platform implements it natively (§8). The rest of the clients are described in [Clients](clients.md), and the components in `libs/design/components.md`.

## 1. Scope

* **Where it applies**: on Apple TV, Android TV and Google TV always; on a computer connected to a television, in the television mode of the web UI, the Mac and the desktop app, a setting under Appearance that the user menu also switches, with ⌃⌘T on the Mac and Shift+F11 elsewhere. In television mode the menu also offers full screen, with the browser's and the Mac's keys, ⌃⌘F and F11.
* **Theme**: it starts dark, as televisions are watched in dim rooms, and follows the user's choice of light or dark after that.
* **Input**: the D-pad moves focus, select activates the focused element, and back goes up a level (§4). On a computer the arrow keys are the D-pad, Return is select, and Escape and Backspace are back. Nothing reacts to hover, and nothing shows a keyboard shortcut, since a remote cannot press it.

## 2. Screen

* **Background**: `color.tvBackground`, a light gray (`palette.neutral.200`) in the light theme, as on Apple TV, for white is glaring on a large screen; and a dark gray (`palette.neutral.900`) rather than black in the dark theme, so that a focused card's shadow shows.
* **Layout**: the `tv` values of the layout tokens: `layout.margin` 80 pt, which keeps content inside the overscan-safe area; `layout.gutter` 40 pt between cards; posters 240 pt wide and thumbnails 480 pt; no sidebar and no touch targets.
* **Type**: the `tv` typography scale, from 52 pt for display and 30 pt for a row's title down to 24 pt for body text and 18 pt for captions.

## 3. Navigation Bar

* **Tabs**: Home, Libraries, Discover (where a server discovers) and Search in a pill centered at the top, on `color.surfaceRaised` at 60 %; the current tab is filled with `color.fillSelected`.
* **Menu button**: three vertical dots in a round button as tall as the tabs, at the screen's right and centered on the tabs vertically, open the user menu (§5); the user's avatar and name head the menu rather than sit beside the tabs.
* **Band**: once the page scrolls, a solid band of the background lies under the bar, so that content scrolled up passes under it rather than through it. It is not blurred, since a blurred edge looks poor on a large screen.
* **Behind the bar**: Home's hero reaches the top of the screen, behind the bar; other pages start below it. A title's details, a discovered title, a library, a catalog and Search hide the bar (§6.2, §6.3, §6.4).
* **Opening tabs**: on a tab's own page, moving focus to another tab opens it, as on tvOS. Deeper pages open a tab only when it is selected, so that moving up to the tabs does not leave the page. Search opens only when selected, from every page, since its page hides the tabs (§6.4).

## 4. Focus

* **Moving**: the D-pad moves focus by rows; elements are in one row when they overlap vertically by at least half the smaller one's height. Up and down move to the previous or next row, to its element nearest the focused one horizontally, preferring those in view. Left and right move along the row; past its first element, left moves to the rightmost element in view of the previous row, and past its last, right moves to the leftmost in view of the next. Above the first row are the tabs, focus going to the current one; below the last row nothing. Regions side by side, such as Search's keyboard and results, have rows of their own, and left and right cross between them at their edges. A choice that focus makes, such as a library's Recommended and All or its filters, is entered at its current element, wherever focus comes from. Where focus cannot move, the focused element shakes sideways for 320 ms.
* **Scrolling**: the page scrolls to keep the focused element in view, stopping 200 pt below the top, so that its row's title shows under the bar, and 120 pt above the bottom, with room for a card to grow, so that the last row reaches the page's end. A row scrolls sideways to keep its focused card in view.
* **Cards**: a focused card scales by `layout.focusScale` (1.05) and its artwork gets a 3 pt `color.accent` border 1.5 pt apart from it and a shadow of `color.tvFocusShadow` with an offset of 12 pt and a blur of 32 pt, over `duration.fast` with `easing.standard`. The other cards of a row are at 75 % opacity. Rows leave room above and below for a card to grow.
* **Buttons and tabs**: a focused button or tab turns white with black content. White belongs to focus alone: a button that is white elsewhere, such as Play over the artwork, is glass like the others until it has focus. A text field with focus gets a white ring of 3 pt.
* **Back**: it always goes up a level, never simply back through the browser's history. An open menu or dialog closes, and focus returns to what opened it. On a tab's page, back moves focus from the page to its tab and returns the page to its top, with every row scrolled back to its start; from a tab, it moves to Home's. Any deeper page — a title's details, a library, Search — returns to where it was opened from, as it was left: the page at the same position, the row scrolled as it was, and focus on the card that opened it. A page opened directly returns to its parent: a library to Libraries, a discovered title or a catalog to Discover, anything else to Home. Opening a tab by focusing it replaces the page rather than adding to the history, so that back never steps through the tabs.

## 5. Menus

* **User menu**: the menu button opens the same menu as on other layouts, 400 pt wide for the larger type and in frosted glass — `color.surfaceOverlay` at 45 % over a strong blur — so that the artwork behind shows through: the appearance, switching users, settings, switching back to the standard layout and, in television mode, full screen. Its items fill with `color.fill` when they have focus, as they do under a pointer elsewhere.
* **Focus within**: while a menu is open, focus moves only among its items and the button that opened it. A menu of choices, such as a library's order, opens with focus on the current one. Back closes it and returns focus to the button.
* **Buttons that open menus**: other than the menu button, they turn white with focus as other buttons do, and keep a `color.fill` highlight while their menu is open and focus is in it.
* **The menu button**: it has two looks only: bare, and, with focus or while its menu is open, in the tabs' glass. Selecting it opens the menu with focus on Settings; back returns focus to it, and selecting it while its menu is open closes the menu.

## 6. Screens

### 6.1 Home
A hero 78 % of the screen's height reaches behind the bar, with the title's logo or name at 72 pt and buttons 64 pt high; the rows follow it. A row on television has no scroll arrows, and its cards are `layout.gutter` apart.

### 6.2 Details
The page hides the navigation bar, and back returns to where it was opened from (§4). The backdrop fills 86 % of the screen's height, with the logo or name at 72 pt, two lines of overview without the tagline, the actions 64 pt high, and the text at most 960 pt wide. Focus starts on the primary action; focus on a season's pill chooses it. The rest is as on other layouts (`libs/design/components.md`, Details Screen).

### 6.3 Grids
A library's items and search results are grids whose columns are at least a poster wide, `layout.gutter` apart. A library hides the navigation bar and the menu button: its name alone heads it at the top left, beside its Recommended and All in a pill that looks like the tabs: the current one filled with `color.fillSelected`, the focused one white. Focus starts on the current one when the library opens. As on the tabs, focus chooses without selecting: moving to Recommended or All shows it, and moving along All's filters — All, Unplayed, Played, Favorites, in a pill alike — applies them. These choices and the order replace the page rather than adding to the history, so that back leaves the library for where it was opened from.

A catalog of Discover is a grid alike, under its name, with focus starting on its first title; a discovered title is laid out as an item's details (§6.2), focus starting on its first action. A row of Discover ends with a See All card, which opens its catalog, in place of the link beside the row's title on other layouts.

### 6.4 Search
* **Without tabs**: the page hides the navigation bar, focus starts on the keyboard, and back leaves the page once the query is empty.
* **Left**: a column 300–480 pt wide that stays in place while the right scrolls. From the top, it holds the query, whose blinking `color.accent` caret shows whether or not it has focus; keys for a space, deleting a character and clearing; and a keyboard of letters and digits in six columns. A computer's keyboard types into it as well. Within it, back deletes a character, and goes back only once there is none.
* **Right**: the larger part shows Home's rows while nothing is typed, and the results by server, in grids, as the query is typed. Its margin is 24 pt, enough for a focused card to grow, and its rows' cards align with their titles.
* **Running**: the search runs as the query is typed, after a pause of 300 ms, and the address follows the query, so that results can be linked to and going back restores the query.

### 6.5 Signing In
A television signs a Mavio account in with a code to enter at mavio.dev/link; a new server's setup shows the server's address to open on a computer or phone ([Accounts §6](accounts.md#6-signing-in)).

## 7. Player

The on-screen controller follows the platform's focus and stays up while it has focus. The track and subtitle pickers open as a side panel.

## 8. Platforms

| Platform | Focus | Implemented in |
| :--- | :--- | :--- |
| Apple TV | The tvOS focus engine | `apps/tvos` |
| Android TV and Google TV | Compose for TV focus | `apps/android-tv` |
| Web UI | Its own focus movement (§4), with the root element's `data-tv` switching the tokens to their `tv` values | `apps/web/src/shell/tv.tsx`, `apps/web/src/shell/spatial.ts`, `apps/web/src/shell/back.ts`, `apps/web/src/styles.css` |
| Mac and desktop | The television mode of each app, with the arrow keys as the D-pad | `apps/macos`, `apps/desktop` |
