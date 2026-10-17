// Package procs records the server's own process and the processes it
// starts — process plugins, rclone and transcoding ffmpeg — in its home,
// so that a server started again ends those a server killed, or still
// running, left behind. A process is known by its ID, start time and
// executable, so that an ID the system gave another process since is
// left alone.
package procs
