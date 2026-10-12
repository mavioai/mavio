package library

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
)

// StoragePolicy is what the server reads of the media of a storage class on
// its own, apart from what playback reads.
type StoragePolicy struct {
	// WholeFiles allows background jobs that read whole files: keyframes
	// read by ffprobe, trickplay sheets decoded from the whole video and
	// loudness measured over the whole track.
	WholeFiles bool
	// Samples allows background jobs that seek to a few points of a video:
	// black border detection and chapter images.
	Samples bool
	// Speculative allows reads no one asked for: head, tail and start
	// position prefetches, keep-alive and wake reads.
	Speculative bool
	// PruneFolders takes the items of folders whose modification time and
	// file ID did not change from the previous scan, without listing them.
	PruneFolders bool
	// ScanConcurrency bounds the folders a scan lists at once; zero leaves
	// it to the device.
	ScanConcurrency int
}

// PolicyOf returns the storage policy of a storage class. Local disks and
// network shares are read freely; on cloud drives, where reads count
// against quotas and rate limits, the server reads only the few bytes it
// needs, and lists folders anew in each scan since their modification
// times do not tell changes below them.
func PolicyOf(class core.StorageClass) StoragePolicy {
	if class == core.StorageCloud {
		return StoragePolicy{ScanConcurrency: 2}
	}
	return StoragePolicy{WholeFiles: true, Samples: true, Speculative: true, PruneFolders: true}
}

// policy returns the storage policy of a file of lib.
func (j *Jobs) policy(lib *core.Library, path string) StoragePolicy {
	if j.Scanner != nil {
		return PolicyOf(j.Scanner.classify(lib, path))
	}
	return PolicyOf(lib.StorageClassOf(path))
}

// itemPolicy returns the storage policy of a file of the library of an
// item.
func (j *Jobs) itemPolicy(ctx context.Context, itemID core.ID, path string) (StoragePolicy, error) {
	it, err := j.Store.Items().Get(ctx, itemID)
	if err != nil {
		return StoragePolicy{}, err
	}
	lib, err := j.Store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return StoragePolicy{}, err
	}
	return j.policy(&lib, path), nil
}
