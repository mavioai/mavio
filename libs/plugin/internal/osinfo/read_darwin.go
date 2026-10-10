package osinfo

func read() (string, string, bool) { return readMac("/System/Library/CoreServices") }
