package storage

import (
	"context"
	"path"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestClassify(t *testing.T) {
	kinds := map[string]DeviceKind{
		"/media":       KindLocalHDD,
		"/mnt/nas":     KindRemoteNAS,
		"/mnt/nas/tv":  KindRemoteNAS,
		"/mnt/cloud":   KindCloudMount,
		"/mnt/rclone":  KindLocalSSD,
		"/mnt/unknown": KindUnknown,
	}
	var detected []string
	c := NewClassifier(&Detector{Detect: func(p string) (DeviceInfo, error) {
		detected = append(detected, p)
		for dir := p; dir != "/"; dir = path.Dir(dir) {
			if k, ok := kinds[dir]; ok {
				return DeviceInfo{ID: dir, Kind: k}, nil
			}
		}
		return DeviceInfo{ID: "/", Kind: KindLocalSSD}, nil
	}})
	lib := &core.Library{
		Paths:          []string{"/media/films", "/mnt/nas", "/mnt/cloud", "/mnt/rclone", "/mnt/unknown"},
		StorageClasses: map[string]core.StorageClass{"/mnt/rclone": core.StorageCloud},
	}
	tests := []struct {
		path string
		want core.StorageClass
	}{
		{"/media/films", core.StorageLocal},
		{"/mnt/nas", core.StorageNetwork},
		{"/mnt/nas/tv/a.mkv", core.StorageNetwork},
		{"/mnt/cloud", core.StorageCloud},
		{"/mnt/cloud/a.mkv", core.StorageCloud},
		{"/mnt/rclone", core.StorageCloud},
		{"/mnt/rclone/a/b.mkv", core.StorageCloud},
		{"/mnt/unknown", core.StorageLocal},
	}
	for _, tt := range tests {
		if got := c.Classify(lib, tt.path); got != tt.want {
			t.Errorf("Classify(%q) = %q, want = %q", tt.path, got, tt.want)
		}
	}
	for _, p := range detected {
		if p == "/mnt/rclone" || p == "/mnt/rclone/a/b.mkv" {
			t.Errorf("detected %q, whose class the library sets", p)
		}
	}
	if got := c.Classes(lib); len(got) != 5 || got["/mnt/nas"] != core.StorageNetwork || got["/mnt/rclone"] != core.StorageCloud {
		t.Errorf("Classes = %v", got)
	}
}

func TestPathClasses(t *testing.T) {
	libs := []core.Library{{Paths: []string{"rclone:pp:Movies"}, StorageClasses: map[string]core.StorageClass{"rclone:pp:Movies": core.StorageNetwork}}}
	p := &PathClasses{
		Classifier: NewClassifier(&Detector{Detect: func(string) (DeviceInfo, error) { return DeviceInfo{Kind: KindLocalHDD}, nil }}),
		Libraries:  func(context.Context) ([]core.Library, error) { return libs, nil },
	}
	for _, tc := range []struct {
		path string
		want core.StorageClass
	}{
		{"rclone:pp:Movies/Film", core.StorageNetwork},
		{"rclone:pp:Downloads", core.StorageCloud},
		{"/srv/downloads", core.StorageLocal},
	} {
		if got := p.Class(t.Context(), tc.path); got != tc.want {
			t.Errorf("Class(%q) = %v, want = %v", tc.path, got, tc.want)
		}
	}
}
