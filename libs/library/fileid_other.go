//go:build !unix

package library

import "io/fs"

// osFileID is empty where the file system exposes no stable ID through
// os.Stat; folders are then compared by modification time alone.
func osFileID(fs.FileInfo) string { return "" }
