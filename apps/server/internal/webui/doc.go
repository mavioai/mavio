// Package webui serves the web UI (apps/web), which its build writes into
// dist and the server embeds, at /: its files as they are, and index.html
// for every other path, which the UI routes itself. A server built
// without the web UI serves a page saying so.
package webui
