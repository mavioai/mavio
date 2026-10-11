package disk

import (
	"io/fs"
	"time"
)

// unixEpochTicks are the .NET ticks of the Unix epoch.
const unixEpochTicks = 621_355_968_000_000_000

// Ticks returns a time as .NET's DateTime.Ticks of its UTC: the 100
// nanoseconds since 0001-01-01.
func Ticks(t time.Time) int64 { return t.UTC().UnixNano()/100 + unixEpochTicks }

// CreationTime returns when a file or folder was created, as .NET's
// CreationTimeUtc: its birth time where the system keeps one, else the
// earlier of its change and modification times.
func CreationTime(fi fs.FileInfo) time.Time {
	if t, ok := creationTime(fi); ok {
		return t
	}
	return fi.ModTime()
}

// IsLocked reports whether another process holds a file exclusively, as
// Radarr's IsFileLocked: on Windows one opened without sharing, elsewhere
// one under an flock.
func IsLocked(path string) bool { return isLocked(path) }
