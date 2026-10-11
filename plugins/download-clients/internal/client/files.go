package client

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
)

// CleanFileName makes a release title a file name as Radarr's
// FileNameBuilder.CleanFileName does by default: colons replaced smartly,
// the characters Windows forbids replaced or dropped.
func CleanFileName(name string) string {
	name = strings.ReplaceAll(name, ": ", " - ")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.NewReplacer(`\`, "+", "/", "+", "<", "", ">", "", "?", "!", "*", "-", "|", "", `"`, "").Replace(name)
	return strings.TrimRight(strings.TrimLeft(name, " ."), " ")
}

// DeleteItemData deletes a download's files, as Radarr's DeleteItemData:
// its folder or file, when there; failures are logged.
func DeleteItemData(ctx context.Context, log *slog.Logger, item Item) {
	if item.OutputPath == "" {
		return
	}
	if _, err := os.Stat(item.OutputPath); errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err := os.RemoveAll(item.OutputPath); err != nil && log != nil {
		log.WarnContext(ctx, "delete download data", "download", item.Title, "path", item.OutputPath, "err", err)
	}
}

// TestFolder checks that a folder exists and, when writable is set, that
// files can be written to it, as Radarr's TestFolder.
func TestFolder(folder, setting string, writable bool) error {
	st, err := os.Stat(folder)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("%s: folder %q does not exist or is inaccessible", setting, folder)
	}
	if !writable {
		return nil
	}
	f, err := os.CreateTemp(folder, ".mavio-write-test-*")
	if err != nil {
		return fmt.Errorf("%s: unable to write to folder %q", setting, folder)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}
