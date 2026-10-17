// Package mpv plays media with libmpv, which it loads at run time with
// purego, without CGO: the player draws into a native view of the window
// (mpv's wid), takes commands from any goroutine and tells of the changes
// of its state from its own.
package mpv
