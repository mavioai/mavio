package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/mavioai/mavio/apps/server/internal/server"
)

// Everything Mavio keeps lives in its home, by default ~/.mavio:
//
//	config.toml   settings, optional
//	mavio.db      the SQLite database
//	plugins/      one folder per plugin
//	metadata/     artwork chosen for items
//	backups/      backups
//	cache/        downloaded and resized images, compiled plugins and,
//	              in transcodes/, the segments of running transcodes
//
// A setting comes from, in order, its flag, its MAVIO_* environment
// variable (MAVIO_CACHE_DIR for cache-dir), config.toml and the layout
// above. Relative paths in config.toml are relative to the home, others
// to the working directory.
const (
	homeName   = ".mavio"
	configName = "config.toml"
	envPrefix  = "MAVIO"
)

// setting is a key of config.toml, which is also a flag.
type setting struct {
	key, usage string
	// path settings name a file or directory.
	path bool
	// def is the default, given the home and the settings resolved
	// before it.
	def func(home string, cfg *server.Config) string
}

// settings are the keys of config.toml; defaults may depend on earlier
// settings.
var settings = []setting{
	{key: "addr", usage: "HTTP listen address", def: constant(":8686")},
	{key: "database", usage: "database: sqlite:<path> or postgres://…", def: func(home string, _ *server.Config) string {
		return "sqlite:" + filepath.Join(home, "mavio.db")
	}},
	{key: "ffmpeg", usage: "ffmpeg binary; without it media is only played directly", def: constant("ffmpeg")},
	{key: "ffprobe", usage: "ffprobe binary; without it libraries are not scanned", def: constant("ffprobe")},
	{key: "cache-dir", path: true, usage: "directory for downloaded and resized images, compiled plugins and transcodes", def: under("cache")},
	{key: "transcode-dir", path: true, usage: "directory for the segments of running transcodes", def: func(_ string, cfg *server.Config) string {
		return filepath.Join(cfg.CacheDir, "transcodes")
	}},
	{key: "plugin-dir", path: true, usage: "directory holding one folder per plugin; empty runs none", def: under("plugins")},
	{key: "metadata-dir", path: true, usage: "directory for the artwork chosen for items of libraries not saving metadata next to their media", def: under("metadata")},
	{key: "backup-dir", path: true, usage: "directory for backups; empty makes none", def: under("backups")},
	{key: "discovery-addr", usage: "UDP address answering discovery requests from clients on the local network; empty answers none", def: constant(":7359")},
	{key: "dev", usage: "serve the development player at /dev/player", def: constant("false")},
	{key: "dev-library", path: true, usage: "add the Movies and Shows folders of this directory as libraries, e.g. .fixtures/dev-library", def: constant("")},
}

func constant(s string) func(string, *server.Config) string {
	return func(string, *server.Config) string { return s }
}

func under(name string) func(string, *server.Config) string {
	return func(home string, _ *server.Config) string { return filepath.Join(home, name) }
}

// options are the settings of a run.
type options struct {
	// Home is the absolute Mavio home.
	Home string
	// Addr is the HTTP listen address.
	Addr   string
	Server server.Config
}

// defineFlags defines the home flag and a flag per setting.
func defineFlags(flags *pflag.FlagSet) {
	flags.String("home", "", "Mavio home holding "+configName+", the database, plugins, metadata, backups and cache (default ~/"+homeName+")")
	for _, s := range settings {
		if s.key == "dev" {
			flags.Bool(s.key, false, s.usage)
			continue
		}
		flags.String(s.key, "", s.usage+defaultHelp(s))
	}
}

// defaultHelp describes a setting's default for its flag's usage.
func defaultHelp(s setting) string {
	switch d := s.def("<home>", &server.Config{CacheDir: "<cache-dir>"}); {
	case d == "":
		return ""
	case strings.Contains(d, "<"):
		return " (default " + filepath.ToSlash(d) + ")"
	default:
		return fmt.Sprintf(" (default %q)", d)
	}
}

// load resolves the settings of a run from flags, MAVIO_* environment
// variables, the home's config.toml and the defaults, and creates the
// directories they name.
func load(flags *pflag.FlagSet) (options, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	// An empty variable empties a setting, such as MAVIO_PLUGIN_DIR= for no plugins.
	v.AllowEmptyEnv(true)
	if err := v.BindPFlags(flags); err != nil {
		return options{}, err
	}
	// fromFile reports whether a setting comes from config.toml, whose
	// relative paths are relative to the home.
	fromFile := func(key string) bool {
		if f := flags.Lookup(key); f != nil && f.Changed {
			return false
		}
		if _, ok := os.LookupEnv(envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))); ok {
			return false
		}
		return v.InConfig(key)
	}

	home := v.GetString("home")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return options{}, fmt.Errorf("locate home: %w; set --home or MAVIO_HOME", err)
		}
		home = filepath.Join(user, homeName)
	}
	home, err := absolute(home, "")
	if err != nil {
		return options{}, err
	}
	if err := os.MkdirAll(home, 0o750); err != nil {
		return options{}, fmt.Errorf("create home: %w", err)
	}

	v.SetConfigFile(filepath.Join(home, configName))
	if err := v.ReadInConfig(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return options{}, fmt.Errorf("read %s: %w", v.ConfigFileUsed(), err)
	}
	for _, key := range v.AllKeys() {
		if v.InConfig(key) && !slices.ContainsFunc(settings, func(s setting) bool { return s.key == key }) {
			return options{}, fmt.Errorf("%s: unknown setting %q", v.ConfigFileUsed(), key)
		}
	}

	o := options{Home: home}
	cfg := &o.Server
	for _, s := range settings {
		v.SetDefault(s.key, s.def(home, cfg))
		value := v.GetString(s.key)
		base := "" // the working directory
		if fromFile(s.key) {
			base = home
		}
		if s.path {
			if value, err = absolute(value, base); err != nil {
				return options{}, fmt.Errorf("%s: %w", s.key, err)
			}
		}
		if file, ok := strings.CutPrefix(value, "sqlite:"); ok && s.key == "database" && file != "" {
			if file, err = absolute(file, base); err != nil {
				return options{}, fmt.Errorf("database: %w", err)
			}
			value = "sqlite:" + file
		}
		if err := assign(&o, s.key, value); err != nil {
			return options{}, err
		}
	}

	dirs := []string{cfg.CacheDir, cfg.TranscodeDir, cfg.PluginDir, cfg.MetadataDir, cfg.BackupDir}
	if file, ok := strings.CutPrefix(cfg.Database, "sqlite:"); ok && file != "" {
		dirs = append(dirs, filepath.Dir(file))
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return options{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return o, nil
}

// assign sets a setting's value.
func assign(o *options, key, value string) error {
	cfg := &o.Server
	targets := map[string]*string{
		"addr": &o.Addr, "database": &cfg.Database, "ffmpeg": &cfg.FFmpeg, "ffprobe": &cfg.FFprobe,
		"cache-dir": &cfg.CacheDir, "transcode-dir": &cfg.TranscodeDir, "plugin-dir": &cfg.PluginDir,
		"metadata-dir": &cfg.MetadataDir, "backup-dir": &cfg.BackupDir, "discovery-addr": &cfg.DiscoveryAddr,
		"dev-library": &cfg.DevLibrary,
	}
	if key == "dev" {
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			cfg.Dev = true
		case "false", "0", "no", "":
			cfg.Dev = false
		default:
			return fmt.Errorf("dev: %q is not a boolean", value)
		}
		return nil
	}
	*targets[key] = value
	return nil
}

// absolute makes path absolute against base, the working directory when
// empty, expanding a leading ~ to the user's home. An empty path stays
// empty.
func absolute(path, base string) (string, error) {
	switch {
	case path == "":
		return "", nil
	case path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`):
		user, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand %s: %w", path, err)
		}
		return filepath.Join(user, path[1:]), nil
	case filepath.IsAbs(path):
		return filepath.Clean(path), nil
	case base != "":
		return filepath.Join(base, path), nil
	default:
		return filepath.Abs(path)
	}
}
