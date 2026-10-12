// Package rclone reaches the remotes of an rclone rcd over its rc API: it
// implements core.Storage for library folders on remotes, reads media through
// the rcd's --rc-serve object server, configures remotes, runs offline
// downloads of cloud drives that offer them, and supervises an rcd the server
// starts itself. It links no rclone code; the rcd may run on another machine.
//
// The package depends only on libs/core and the standard library.
package rclone
