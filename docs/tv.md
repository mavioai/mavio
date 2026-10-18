# Mavio on Television

> English | [简体中文](tv.zh-CN.md)

> The television layout of Mavio's clients: the Apple TV app, the Android TV and Google TV app, and the television mode of the web UI, the Mac app and the desktop app. It is watched from across a room and driven by a remote alone, so it has larger type and margins, focus in place of a pointer, and no hover. This document specifies it once; each platform implements it natively (§8). The rest of the clients are described in [Clients](clients.md), and the components in `libs/design/components.md`.

## 1. Scope

* **Where it applies**: on Apple TV, Android TV and Google TV always; on a computer connected to a television, in the television mode of the web UI, the Mac and the desktop app, a setting under Appearance that the user menu also switches, with ⌃⌘T on the Mac and Shift+F11 elsewhere. In television mode the menu also offers full screen, with the browser's and the Mac's keys, ⌃⌘F and F11.
* **Theme**: it starts dark, as televisions are watched in dim rooms, and follows the user's choice of light or dark after that.
* **Input**: the D-pad moves focus, select activates the focused element, and back goes up a level (§4). On a computer the arrow keys are the D-pad, Return is select, and Escape and Backspace are back. Nothing reacts to hover, and nothing shows a keyboard shortcut, since a remote cannot press it. Nor does anything open a web page: trailers on the web and links to other sites are left out, and logos and ratings that link elsewhere only show.

## 2. Screen

* **Background**: `color.tvBackground`, a light gray (`palette.neutral.200`) in the light theme, as on Apple TV, for white is glaring on a large screen; and a dark gray (`palette.neutral.900`) rather than black in the dark theme, so that a focused card's shadow shows.
* **Layout**: the `tv` values of the layout tokens: `layout.margin` 80 pt, which keeps content inside the overscan-safe area; `layout.gutter` 40 pt between cards; posters 240 pt wide and thumbnails 480 pt; no sidebar and no touch targets.
* **Type**: the `tv` typography scale, from 52 pt for display and 30 pt for a row's title down to 24 pt for body text and 18 pt for captions.

## 3. Navigation Bar

* **Tabs**: Home, Libraries, Discover (where a server discovers) and Search in a pill centered at the top, on `color.surfaceRaised` at 60 %; the current tab is filled with `color.fillSelected`.
* **Menu button**: three vertical dots in a round button of the tabs' glass, as large as the assistant's beside it, at the screen's right and centered on the tabs vertically, open the user menu (§5) as the button is pressed, never as focus moves from it; with focus it is white as the assistant's, and while its menu is open, focus in the menu, it is as without focus; the user's avatar and name head the menu rather than sit beside the tabs.
* **Assistant button**: a microphone in a round button like the menu button, to its left, opens the assistant's panel ([Assistant §9](assistant.md#9-clients)); where the remote has a search or assistant key the app receives, that key opens it from any page.
* **Band**: once the page scrolls, a solid band of the background lies under the bar, so that content scrolled up passes under it rather than through it. It is not blurred, since a blurred edge looks poor on a large screen.
* **Behind the bar**: Home's hero reaches the top of the screen, behind the bar; other pages start below it. A title's details, a discovered title, a library, a catalog and Search hide the bar (§6.2, §6.3, §6.4).
* **Opening tabs**: on a tab's own page, moving focus to another tab opens it, as on tvOS. Deeper pages open a tab only when it is selected, so that moving up to the tabs does not leave the page. Search opens only when selected, from every page, since its page hides the tabs (§6.4).

## 4. Focus

* **Always present**: every page but the player always has one element with focus, and shows which: a page opens with focus on its first element, unless it names another, and focus that cannot return where it was, or whose element goes, as a request deleted, goes to the element nearest it. A message from another device takes focus as it shows, back dismissing it, and gives it back as it goes. The player alone may have none (§7).
* **Moving**: the D-pad moves focus by rows; elements are in one row when they overlap vertically by at least half the smaller one's height. Up and down move to the previous or next row, to its element nearest the focused one horizontally, preferring those in view. Left and right move along the row; past its first element, left moves to the rightmost element in view of the previous row, and past its last, right moves to the leftmost in view of the next. Above the first row are the tabs, focus going to the current one; below the last row nothing. Regions side by side, such as Search's keyboard and results, have rows of their own, and left and right cross between them at their edges. A choice that focus makes, such as a library's Recommended and All or its filters, is entered at its current element, wherever focus comes from. In a text field, left and right move the caret, and in a text box up and down move between its lines; at the field's edges they move focus. A slider's left and right change its value; up and down move focus on from it, as from a number field, and from a field with suggestions or a list of options while its list is closed. In a dialog the D-pad moves focus among its elements. Where focus cannot move, the focused element shakes sideways for 320 ms.
* **Scrolling**: the page scrolls to keep the focused element in view, stopping 200 pt below the top, so that its row's title shows under the bar, and 120 pt above the bottom, with room for a card to grow, so that the last row reaches the page's end. Focus on an element within the page's first screen, as a hero's buttons, returns the page to its top, showing all above it. A row scrolls sideways to keep its focused card in view.
* **Cards**: a focused card scales by `layout.focusScale` (1.05) and its artwork gets a 3 pt `color.accent` border 1.5 pt apart from it and a shadow of `color.tvFocusShadow` with an offset of 12 pt and a blur of 32 pt, over `duration.fast` with `easing.standard`. The other cards of a row are at 75 % opacity. Rows leave room above and below for a card to grow.
* **Buttons and tabs**: a focused button or tab turns white with black content. White belongs to focus alone: a button that is white elsewhere, such as Play over the artwork, is glass like the others until it has focus. A text field with focus gets a white ring of 3 pt. Any other element with focus, as a row of the settings or a person of the cast, turns white with black content as well, its corners rounded, over a shadow; a row of a group, as the settings' are, fills its row exactly, its corners rounded 14 pt as the group's are.
* **Typing**: select on a text or number field opens a keyboard over the page, in a dialog named by the field: what is typed, its blinking `color.accent` caret, a password as dots; for text, keys of digits and letters with `-`, `_`, `.` and `@` in ten columns, which capitals and a set of symbols replace, a space and, in a text box, a new line; for a number, a keypad of digits and a minus, with a point when it may have a fraction; and keys deleting a character, clearing, and Done, in `color.accent`. What it types goes into the field as it is typed, so that a filter filters at once; Done keeps it and returns focus to the field, and back deletes a character, or, once there is none, puts back what the field held and closes. A computer's keyboard types into the keyboard as well, and into a focused field directly, after which select and Backspace are the field's own, sending what was typed and deleting; until then back goes up a level from a field as from anything else. Select on a field with a list of options, as a plugin's indexer definition, opens the list instead.
* **Back**: it always goes up a level, never simply back through the browser's history. An open menu or dialog closes, and focus returns to what opened it. On a tab's page, back moves focus from the page to its tab and returns the page to its top, with every row scrolled back to its start; from a tab, it moves to Home's. Any deeper page — a title's details, a library, Search — returns to where it was opened from, as it was left: the page at the same position, the row scrolled as it was, and focus on the card that opened it. A page opened directly returns to its parent: a title's cast or crew to the title, a section of the settings to its menu or their list, a menu to the list, a library to Libraries, a discovered title, a catalog or the requests to Discover, anything else to Home. Pages show no link back, which the remote's back takes the place of. On the PIN pad of switching users, back deletes a digit, and closes the pad once there is none. Opening a tab by focusing it replaces the page rather than adding to the history, so that back never steps through the tabs.

## 5. Menus

* **User menu**: the menu button opens the same menu as on other layouts, 400 pt wide for the larger type and in frosted glass — `color.surfaceOverlay` at 45 % over a strong blur — so that the artwork behind shows through: the appearance, switching users, settings, switching back to the standard layout and, in television mode, full screen. Its items fill with `color.fill` when they have focus, as they do under a pointer elsewhere.
* **Focus within**: while a menu is open, focus moves only among its items and the button that opened it. A menu of choices, such as a library's order, opens with focus on the current one. Back closes it and returns focus to the button. A select, as the server of the settings, opens its options as such a menu: below it, in the same glass and type, only as it is pressed, with focus on the chosen one; up and down from it move focus on.
* **Buttons that open menus**: other than the menu button, they turn white with focus as other buttons do, and keep a `color.fill` highlight while their menu is open and focus is in it.
* **The menu button**: it has two looks only: bare, and, with focus or while its menu is open, in the tabs' glass. Selecting it opens the menu with focus on Settings; back returns focus to it, and selecting it while its menu is open closes the menu.

## 6. Screens

### 6.1 Home
A hero 78 % of the screen's height reaches behind the bar, with the title's logo or name at 72 pt and buttons 64 pt high; the rows follow it. A row on television has no scroll arrows, and its cards are `layout.gutter` apart.

### 6.2 Details
The page hides the navigation bar, and back returns to where it was opened from (§4). The backdrop fills 86 % of the screen's height, with the logo or name at 72 pt, two lines of overview without the tagline, the actions 64 pt high, and the text at most 960 pt wide. Focus starts on the primary action; focus on a season's pill chooses it. The cast's row ends with a See All card, which opens the whole cast and crew, as a row of Discover (§6.3). Its details, the overview beside the box of facts, are a region in the page's flow with the two as columns, each with rows of its own: down from the leading crew reaches View Full Crew only from a row's last person, passing it from the others, and right crosses into the box only from a row's last person and from View Full Crew, left returning to the nearest of them; between the leading crew and the keywords or tags under them, up and down land on the first of the row they reach. Down from either column's last row leaves the region, and up into it lands on the first element of the overview's last row. A discovered title's overview is laid out alike. The rest is as on other layouts (`libs/design/components.md`, Details Screen).

### 6.3 Grids
A library's items and search results are grids whose columns are at least a poster wide, `layout.gutter` apart. A library hides the navigation bar and the menu button: its name alone heads it at the top left, beside its Recommended and All in a pill that looks like the tabs: the current one filled with `color.fillSelected`, the focused one white. Focus starts on the current one when the library opens. As on the tabs, focus chooses without selecting: moving to Recommended or All shows it, and moving along All's filters — All, Unplayed, Played, Favorites, in a pill alike — applies them. These choices and the order replace the page rather than adding to the history, so that back leaves the library for where it was opened from.

A catalog of Discover is a grid alike, under its name, with focus starting on its first title; a discovered title is laid out as an item's details (§6.2), focus starting on its first action. Its top is three regions in the page's flow, each moving within itself as above: the facts and genres, the actions, and the overview with the box of facts beside it. Down from one moves to the next one's first element — from the genres to Request, from the actions to the overview's first — and down from the overview to the cast's first; up moves back the same way, to the first element of the last row of the region above, of the overview's left column. A row of Discover ends with a See All card, which opens its catalog, in place of the link beside the row's title on other layouts.

### 6.4 Search
* **Without tabs**: the page hides the navigation bar, focus starts on the keyboard, and back leaves the page once the query is empty.
* **Left**: a column 300–480 pt wide that stays in place while the right scrolls. From the top, it holds the query, whose blinking `color.accent` caret shows whether or not it has focus; keys for a space, deleting a character and clearing; and a keyboard of letters and digits in six columns. A computer's keyboard types into it as well. Within it, back deletes a character, and goes back only once there is none.
* **Right**: the larger part shows Home's rows while nothing is typed, and the results by server, in grids, as the query is typed. Its margin is 24 pt, enough for a focused card to grow, and its rows' cards align with their titles.
* **Running**: the search runs as the query is typed, after a pause of 300 ms, and the address follows the query, so that results can be linked to and going back restores the query.

### 6.5 Signing In
A television signs a Mavio account in with a code to enter at mavio.dev/link; a new server's setup shows the server's address to open on a computer or phone ([Accounts §6](accounts.md#6-signing-in)).

### 6.6 Settings
The settings show the list of their sections — the account, the users to switch to, the sections and menus by group — and, when one is selected, a section in its place, or the list of a menu's sections, with no sidebar. Back returns from a section to its menu or the list and from a menu to the list, with focus on the entry that opened it, and from the list to the page the settings were opened from. A page within a section, as a plugin, a profile or a library, returns to the section, and a page within that, as a custom format, to the page it lies in, each as it was shown, as the plugins on their tab; back goes up past pages beside the one it leaves, as a plugin's tabs or a link of its trail opened, never back through them, and a page opened directly returns to the one it lies in.

## 7. Player

The on-screen controller follows the platform's focus and stays up while it has focus. The track and subtitle pickers open as a side panel.

* **Focus**: while the controller is hidden nothing has focus, and left and right seek by 10 seconds; up, down and select show it with focus on Play. Left and right then move focus among the controls, and the controller hides as usual once nothing is pressed for 3 seconds during playback, taking focus with it. The side panel opens with focus on its current choice, and closing it returns focus to the button that opened it.
* **Back**: back closes the side panel, then hides the controller, then leaves the player for where it was opened from, else the item's details.

## 8. Platforms

| Platform | Focus | Implemented in |
| :--- | :--- | :--- |
| Apple TV | The tvOS focus engine | `apps/tvos` |
| Android TV and Google TV | Compose for TV focus | `apps/android-tv` |
| Web UI | Its own focus movement (§4), with the root element's `data-tv` switching the tokens to their `tv` values | `apps/web/src/shell/tv.tsx`, `apps/web/src/shell/spatial.ts`, `apps/web/src/shell/back.ts`, `apps/web/src/styles.css` |
| Mac and desktop | The television mode of each app, with the arrow keys as the D-pad | `apps/macos`, `apps/desktop` |
