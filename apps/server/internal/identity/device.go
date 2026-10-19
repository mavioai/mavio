package identity

import (
	"crypto/sha1" //nolint:gosec // UUIDv5 is defined over SHA-1 (RFC 9562 §5.5).
	"fmt"
	"strings"
)

// deviceNamespace is the UUIDv5 namespace of Mavio's device IDs.
var deviceNamespace = [16]byte{0x29, 0xe1, 0x04, 0xe5, 0xc8, 0x69, 0x47, 0x2d, 0xaf, 0x03, 0xa3, 0xf4, 0xd6, 0xa0, 0x43, 0xf0}

// DeviceID returns the ID that plugins give the services they sign in to
// for this installation, such as OpenAI's agent host ID: a UUID derived
// from the machine's hardware ID — Linux's machine-id, macOS's
// IOPlatformUUID, Windows' MachineGuid — or, in a container or where the
// machine has none, from the server's ID. Derived, it names the machine to
// Mavio's plugins alone, not giving the hardware ID away
// (docs/plugins.md §6).
func DeviceID(serverID string) string {
	return deviceID(machineID(), serverID)
}

// deviceID derives the device ID from a hardware ID, or from the server's
// ID when machine is empty.
func deviceID(machine, serverID string) string {
	name := "server:" + serverID
	if machine = strings.ToLower(strings.TrimSpace(machine)); machine != "" {
		name = "machine:" + machine
	}
	return uuidV5(deviceNamespace, name)
}

// uuidV5 returns the name-based UUID of name in namespace (RFC 9562 §5.5).
func uuidV5(namespace [16]byte, name string) string {
	h := sha1.New() //nolint:gosec // See the import.
	h.Write(namespace[:])
	h.Write([]byte(name))
	var u [16]byte
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x50
	u[8] = u[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// usable reports whether a hardware ID read is one: not empty, nor
// systemd's placeholder of a machine-id not yet set, nor all zeros.
func usable(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && id != "uninitialized" && strings.Trim(id, "0-") != ""
}
