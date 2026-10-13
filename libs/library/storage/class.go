package storage

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Class returns the storage class of the device: cloud mounts are cloud,
// network shares network, and every other device local.
func (d DeviceInfo) Class() core.StorageClass {
	switch d.Kind {
	case KindCloudMount:
		return core.StorageCloud
	case KindRemoteNAS:
		return core.StorageNetwork
	}
	return core.StorageLocal
}

// Classifier classifies the files and folders of libraries.
type Classifier struct {
	devices *Detector
}

// NewClassifier returns a classifier detecting devices with devices; nil
// detects with a Detector of its own.
func NewClassifier(devices *Detector) *Classifier {
	if devices == nil {
		devices = &Detector{}
	}
	return &Classifier{devices: devices}
}

// Classify returns the storage class of path, a file or folder of lib: the
// class the library sets for the path holding it, or else the class of the
// device holding it.
func (c *Classifier) Classify(lib *core.Library, path string) core.StorageClass {
	if class := lib.StorageClassOf(path); class != core.StorageAuto {
		return class
	}
	if core.IsRemotePath(path) {
		return core.StorageCloud
	}
	isRoot := func(root string) bool {
		name, ok := core.StorageName(root, path)
		return ok && name == "."
	}
	if slices.ContainsFunc(lib.Paths, isRoot) {
		// A library path may be a mount point: detect the folder itself.
		return c.devices.Folder(path).Class()
	}
	return c.devices.Device(path).Class()
}

// pathsTTL is how long PathClasses keeps the libraries it read.
const pathsTTL = time.Minute

// PathClasses classifies any media path by the library holding it, for
// mechanisms that know paths but not libraries.
type PathClasses struct {
	// Classifier classifies the paths of a library; nil classifies with a
	// Detector of its own.
	Classifier *Classifier
	// Libraries lists the libraries; they are listed again a minute later.
	Libraries func(ctx context.Context) ([]core.Library, error)

	mu     sync.Mutex
	libs   []core.Library
	loaded time.Time
}

// Class returns the storage class of path: as the library holding it
// classifies it, or, when no library holds it, cloud for paths of rclone
// remotes and the class of its device for other paths.
func (p *PathClasses) Class(ctx context.Context, path string) core.StorageClass {
	p.mu.Lock()
	if p.Classifier == nil {
		p.Classifier = NewClassifier(nil)
	}
	if p.Libraries != nil && time.Since(p.loaded) > pathsTTL {
		if libs, err := p.Libraries(ctx); err == nil {
			p.libs, p.loaded = libs, time.Now()
		}
	}
	var lib *core.Library
	for i := range p.libs {
		if slices.ContainsFunc(p.libs[i].Paths, func(root string) bool { return core.InFolder(path, root) }) {
			lib = &p.libs[i]
			break
		}
	}
	c := p.Classifier
	p.mu.Unlock()
	switch {
	case lib != nil:
	case core.IsRemotePath(path):
		return core.StorageCloud
	default:
		return c.devices.Device(path).Class()
	}
	return c.Classify(lib, path)
}

// Classes returns the storage class of each path of lib.
func (c *Classifier) Classes(lib *core.Library) map[string]core.StorageClass {
	classes := make(map[string]core.StorageClass, len(lib.Paths))
	for _, p := range lib.Paths {
		classes[p] = c.Classify(lib, p)
	}
	return classes
}
