package cardigann

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strings"
)

// Blocklist are the definitions Prowlarr implements in code instead.
var Blocklist = []string{
	"aither", "animeworld", "audiobookbay", "beyond-hd-oneurl", "beyond-hd",
	"blutopia", "brsociety", "danishbytes", "datascene", "desitorrents",
	"hdbits", "lat-team", "mteamtp", "mteamtp2fa", "reelflix",
	"shareisland", "skipthecommercials", "tellytorrent",
}

// CustomDir is the directory of custom definitions, under the
// definitions' directory.
const CustomDir = "Custom"

// LoadDefinitions reads the definitions of a directory, as Prowlarr's
// IndexerDefinitionUpdateService reads its Definitions folder: its *.yml
// files but those of the blocklist, then those of its Custom directory.
// Definitions repeating another's file name or name, and those that do
// not parse, are logged and left out.
func LoadDefinitions(fsys fs.FS, log *slog.Logger) ([]*Definition, error) {
	if log == nil {
		log = slog.Default()
	}
	var defs []*Definition
	for _, dir := range []string{".", CustomDir} {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			if dir == CustomDir && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read definitions: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".yml") {
				continue
			}
			file := strings.TrimSuffix(name, ".yml")
			if dir == "." && slices.Contains(Blocklist, file) {
				continue
			}
			data, err := fs.ReadFile(fsys, path.Join(dir, name))
			if err != nil {
				return nil, fmt.Errorf("read definition %s: %w", name, err)
			}
			def, err := Parse(data)
			if err != nil {
				log.Error("cardigann: error while parsing definition", "file", path.Join(dir, name), "error", err)
				continue
			}
			def.File = file
			if slices.ContainsFunc(defs, func(d *Definition) bool { return d.File == def.File || d.Name == def.Name }) {
				log.Warn("cardigann: definition does not have a unique file name or indexer name", "file", path.Join(dir, name))
				continue
			}
			defs = append(defs, def)
		}
	}
	return defs, nil
}
