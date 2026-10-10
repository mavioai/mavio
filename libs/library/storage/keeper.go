package storage

import (
	"sync"
	"time"
)

// DefaultWakeCooldown is how long after waking a volume another wake of
// it is skipped, as khuaplayer's recent-file warming does: a wake is a
// single read, not a keep-alive.
const DefaultWakeCooldown = 30 * time.Second

// Keeper keeps the volumes media plays from awake and wakes the volume of
// media a client is about to play, so that playback does not wait for a
// disk to spin up or a share to reconnect. Only hard disks and network
// volumes are read: solid-state drives do not sleep, and reading cloud
// placeholders would download them.
type Keeper struct {
	// Devices detects devices; nil detects with a Detector of its own.
	Devices *Detector
	// Interval spaces the heartbeats of a volume; default
	// DefaultHeartbeatInterval.
	Interval time.Duration
	// WakeCooldown spaces the wakes of a volume; default
	// DefaultWakeCooldown.
	WakeCooldown time.Duration
	// Now returns the current time; default time.Now.
	Now func() time.Time
	// Read is a heartbeat read; default VolumeHeartbeat. Prefetch warms
	// a file about to be opened; default PrefetchHeadTail with the
	// default budgets.
	Read, Prefetch func(path string) error

	once     sync.Once
	beats    *Heartbeats
	wakes    *Heartbeats
	mu       sync.Mutex
	inflight map[string]bool
	wg       sync.WaitGroup
}

func (k *Keeper) init() {
	k.once.Do(func() {
		if k.Devices == nil {
			k.Devices = &Detector{}
		}
		k.beats = &Heartbeats{Interval: k.Interval}
		wake := k.WakeCooldown
		if wake <= 0 {
			wake = DefaultWakeCooldown
		}
		k.wakes = &Heartbeats{Interval: wake}
		if k.Read == nil {
			k.Read = VolumeHeartbeat
		}
		if k.Prefetch == nil {
			k.Prefetch = func(path string) error { return PrefetchHeadTail(path, 0, 0) }
		}
		k.inflight = map[string]bool{}
	})
}

func (k *Keeper) now() time.Time {
	if k.Now != nil {
		return k.Now()
	}
	return time.Now()
}

// Sleeps reports whether a device spins down or disconnects when idle.
func (d DeviceInfo) Sleeps() bool {
	if d.ID == "" || d.Kind == KindCloudMount {
		return false
	}
	return d.Rotational || d.Remote || d.Kind == KindLocalHDD || d.Kind == KindRemoteNAS
}

// Beat reads each volume holding one of paths that sleeps and is due a
// heartbeat, once whatever the number of its paths, and returns how many
// reads it started. Reads run in the background; a volume whose last read
// has not returned, such as a hung share, is skipped.
func (k *Keeper) Beat(paths ...string) int {
	k.init()
	now := k.now()
	started := 0
	seen := map[string]bool{}
	for _, p := range paths {
		dev := k.Devices.Device(p)
		if !dev.Sleeps() || seen[dev.ID] {
			continue
		}
		seen[dev.ID] = true
		if k.busy(dev.ID) || !k.beats.Claim(dev.ID, now) {
			continue
		}
		k.start(dev.ID, func() { _ = k.Read(p) })
		started++
	}
	return started
}

// Wake reads the volume holding path, unless it does not sleep or was
// woken within the cooldown, and warms the file's container header and
// index. It reports whether it started; the reads run in the background.
func (k *Keeper) Wake(path string) bool {
	k.init()
	dev := k.Devices.Device(path)
	if !dev.Sleeps() || k.busy(dev.ID) || !k.wakes.Claim(dev.ID, k.now()) {
		return false
	}
	k.start(dev.ID, func() {
		if k.Read(path) == nil {
			_ = k.Prefetch(path)
		}
	})
	return true
}

func (k *Keeper) busy(volume string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.inflight[volume]
}

func (k *Keeper) start(volume string, read func()) {
	k.mu.Lock()
	k.inflight[volume] = true
	k.mu.Unlock()
	k.wg.Go(func() {
		defer func() {
			k.mu.Lock()
			delete(k.inflight, volume)
			k.mu.Unlock()
		}()
		read()
	})
}

// Wait waits for the reads started to return.
func (k *Keeper) Wait() { k.wg.Wait() }
