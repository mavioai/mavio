package osinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadRelease(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		wantName    string
		wantVersion string
		wantOK      bool
	}{
		{
			name: "os-release",
			files: map[string]string{
				"os-release": "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04.4 LTS\"\n",
				"hostname":   "ID=other\n",
			},
			wantName: "ubuntu", wantVersion: `"22.04"`, wantOK: true,
		},
		{
			name:     "later files win",
			files:    map[string]string{"alpine-release": "3.19.1\n", "os-release": "ID=alpine\r\nVERSION_ID=3.19.1\r\n"},
			wantName: "alpine", wantVersion: "3.19.1", wantOK: true,
		},
		{name: "no release files", files: map[string]string{"hosts": "127.0.0.1 localhost\n"}, wantName: "Linux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, data := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			name, version, ok := readRelease(dir)
			if name != tt.wantName || version != tt.wantVersion || ok != tt.wantOK {
				t.Errorf("readRelease() = %q, %q, %v, want = %q, %q, %v", name, version, ok, tt.wantName, tt.wantVersion, tt.wantOK)
			}
		})
	}
}

func TestReadMac(t *testing.T) {
	plist := func(v string) string {
		return "<dict>\n\t<key>ProductVersion</key>\n\t<string>" + v + "</string>\n</dict>\n"
	}
	tests := []struct {
		name, file, data      string
		wantName, wantVersion string
		wantOK                bool
	}{
		{name: "macOS 14", file: "SystemVersion.plist", data: plist("14.5"), wantName: "macOS", wantVersion: "14.5", wantOK: true},
		{name: "unmatched version", file: "SystemVersion.plist", data: plist("27.0.1"), wantName: "macOS", wantVersion: "10.0", wantOK: true},
		{name: "server", file: "ServerVersion.plist", data: plist("10.15.7"), wantName: "macOS Server", wantVersion: "10.15.7", wantOK: true},
		{name: "no list", file: "Other.plist", data: plist("14.5")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.data), 0o644); err != nil {
				t.Fatal(err)
			}
			name, version, ok := readMac(dir)
			if name != tt.wantName || version != tt.wantVersion || ok != tt.wantOK {
				t.Errorf("readMac() = %q, %q, %v, want = %q, %q, %v", name, version, ok, tt.wantName, tt.wantVersion, tt.wantOK)
			}
		})
	}
}

func TestGet(t *testing.T) {
	name, _ := Get()
	if name == "" {
		t.Error("Get() name is empty")
	}
}
