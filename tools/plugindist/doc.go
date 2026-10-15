// Package plugindist packages the first-party plugins of plugins/ as a
// catalog offers them: a zip of each plugin's folder, its manifest.json
// and executable, for every platform a process plugin is built for, and
// the catalog.json listing them with their SHA-256 digests. The command
// cmd/plugindist runs it.
package plugindist
