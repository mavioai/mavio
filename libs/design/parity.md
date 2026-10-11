# Feature Parity

> English | [简体中文](parity.zh-CN.md)

The state of every feature on each client ([Clients §6.8](../../docs/clients.md#68-galleries-and-visual-review)). A client lacks a feature only by decision: a feature not applicable to a client says why below the table, and a pull request adding a feature to one client updates this table.

✓ shipped · ○ planned · — not applicable

| Feature | Web | iPhone, iPad | Apple TV | Mac | Android | Android TV | Windows, Linux |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **Servers and sign-in** | | | | | | | |
| Server discovery on the local network | — | ○ | ○ | ○ | ○ | ○ | ○ |
| Several servers signed in at once | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Sign-in with a password | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Quick Connect | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| **Browsing** | | | | | | | |
| Home: hubs merged across servers | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Library Recommended tab | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Libraries with sort and filters | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Search | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Details: movie, series, season, person, collection | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| More like this | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Favorites, played and unplayed | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Live updates from the event stream | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| **Playback** | | | | | | | |
| Direct play, remux and transcode | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Second engine (libmpv) | — | ○ | ○ | ○ | ○ | ○ | — |
| HDR10 and Dolby Vision output | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Audio and subtitle selection | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Styled ASS subtitles | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Subtitle offset | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Playback speed | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Chapters and trickplay thumbnails | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Next episode | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Picture in Picture | ○ | ○ | ○ | ○ | ○ | — | — |
| AirPlay | ○ | ○ | — | ○ | — | — | — |
| Open in an external player | ○ | — | — | — | — | — | — |
| System media controls and media keys | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Progress reporting, offline replay | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| **Offline** | | | | | | | |
| Downloads | — | ○ | — | ○ | ○ | — | ○ |
| Starting on cached content | — | ○ | ○ | ○ | ○ | ○ | ○ |
| **Television home screen** | | | | | | | |
| Top Shelf | — | — | ○ | — | — | — | — |
| Watch Next and channels | — | — | — | — | — | ○ | — |
| **Requests and administration** | | | | | | | |
| Discovery and requests | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Administration | ○ | — | — | — | — | — | — |
| First-run setup | ○ | — | — | — | — | — | — |

## Not Applicable

* **Server discovery and starting on cached content on the web**: browsers cannot discover servers on the local network, and the web UI is loaded from a server, which it reaches before anything else.
* **Second engine on the web and on Windows and Linux**: the web plays with the browser's player; Windows and Linux play with libmpv alone.
* **Picture in Picture on Android TV and on Windows and Linux**: Android TV and Google TV devices do not offer it; the desktop app has windows of its own instead.
* **AirPlay on Apple TV, Android and Windows and Linux**: Apple TV receives AirPlay rather than sending it; the others lack it.
* **Open in an external player outside the web**: the apps direct play with libmpv what a browser cannot.
* **Downloads on the web and on television**: browsers have no background transfer and storage for media; television apps stream.
* **Television home screen elsewhere**: each is its platform's own.
* **Administration and first-run setup outside the web**: they are done in the web UI, which every server serves and which needs no installation.
