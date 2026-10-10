package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// run loads the settings of a run with args, after writing toml to the
// home's config.toml when not empty.
func run(t *testing.T, home, toml string, args ...string) (options, error) {
	t.Helper()
	if toml != "" {
		if err := os.MkdirAll(home, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, configName), []byte(toml), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	flags := pflag.NewFlagSet("mavio", pflag.ContinueOnError)
	defineFlags(flags)
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	return load(flags)
}

func TestLoadDefaults(t *testing.T) {
	user := t.TempDir()
	t.Setenv("HOME", user)
	t.Setenv("USERPROFILE", user)
	o, err := run(t, "", "")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(user, ".mavio")
	cfg := o.Server
	for _, tc := range []struct{ name, got, want string }{
		{"home", o.Home, home},
		{"addr", o.Addr, ":8686"},
		{"database", cfg.Database, "sqlite:" + filepath.Join(home, "mavio.db")},
		{"ffmpeg", cfg.FFmpeg, "ffmpeg"},
		{"cache-dir", cfg.CacheDir, filepath.Join(home, "cache")},
		{"transcode-dir", cfg.TranscodeDir, filepath.Join(home, "cache", "transcodes")},
		{"plugin-dir", cfg.PluginDir, filepath.Join(home, "plugins")},
		{"metadata-dir", cfg.MetadataDir, filepath.Join(home, "metadata")},
		{"backup-dir", cfg.BackupDir, filepath.Join(home, "backups")},
		{"discovery-addr", cfg.DiscoveryAddr, ":7359"},
		{"dev-library", cfg.DevLibrary, ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if cfg.Dev {
		t.Error("dev = true, want false")
	}
	for _, dir := range []string{"cache/transcodes", "plugins", "metadata", "backups"} {
		if fi, err := os.Stat(filepath.Join(home, dir)); err != nil || !fi.IsDir() {
			t.Errorf("%s not created: %v", dir, err)
		}
	}
}

func TestLoadPrecedence(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	home := filepath.Join(t.TempDir(), "home")
	elsewhere := t.TempDir()
	toml := strings.Join([]string{
		`addr = ":9000"`,
		fmt.Sprintf("database = %q", "sqlite:"+filepath.Join(elsewhere, "db", "library.db")),
		fmt.Sprintf("cache-dir = %q", filepath.Join(elsewhere, "unused")),
		fmt.Sprintf("plugin-dir = %q", filepath.Join(elsewhere, "extensions")),
		`backup-dir = ""`,
		`metadata-dir = "art"`,
		`dev = true`,
	}, "\n")
	t.Setenv("MAVIO_HOME", home)
	t.Setenv("MAVIO_METADATA_DIR", "env-art")
	t.Setenv("MAVIO_ADDR", ":9100")
	o, err := run(t, home, toml, "--addr", ":9200", "--cache-dir", "cache")
	if err != nil {
		t.Fatal(err)
	}
	cfg := o.Server
	for _, tc := range []struct{ name, got, want string }{
		{"home from the variable", o.Home, home},
		{"addr: the flag before the variable and file", o.Addr, ":9200"},
		{"database from the file", cfg.Database, "sqlite:" + filepath.Join(elsewhere, "db", "library.db")},
		{"cache-dir: a relative flag is relative to the working directory", cfg.CacheDir, filepath.Join(work, "cache")},
		{"transcode-dir follows cache-dir", cfg.TranscodeDir, filepath.Join(work, "cache", "transcodes")},
		{"plugin-dir from the file", cfg.PluginDir, filepath.Join(elsewhere, "extensions")},
		{"backup-dir: emptied by the file", cfg.BackupDir, ""},
		{"metadata-dir: the variable before the file, whose relative path is then not checked", cfg.MetadataDir, filepath.Join(work, "env-art")},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if !cfg.Dev {
		t.Error("dev = false, want true from the file")
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "db")); err != nil {
		t.Errorf("database folder not created: %v", err)
	}
}

func TestLoadRejects(t *testing.T) {
	for _, tc := range []struct{ name, toml string }{
		{"unknown setting", `plugins-dir = "x"`},
		{"home in the file", `home = "/elsewhere"`},
		{"nested table", "[server]\naddr = \":1\""},
		{"malformed", `addr = `},
		{"not a boolean", `dev = "maybe"`},
		{"relative folder", `plugin-dir = "plugins"`},
		{"relative database", `database = "sqlite:mavio.db"`},
		{"home-relative folder", `cache-dir = "~/cache"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if _, err := run(t, home, tc.toml, "--home", home); err == nil {
				t.Error("load = nil error, want one")
			}
		})
	}
}
