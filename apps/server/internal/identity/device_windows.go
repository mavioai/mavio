package identity

import "golang.org/x/sys/windows/registry"

// machineID returns Windows' MachineGuid, or empty when it cannot be read.
func machineID() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	id, _, err := k.GetStringValue("MachineGuid")
	if err != nil || !usable(id) {
		return ""
	}
	return id
}
