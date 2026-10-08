// Package plugin is the Mavio plugin SDK and host runtime. Plugins implement
// the services in mavio.plugin.v1 and run either as WebAssembly modules on
// wazero or as child processes reached over a Unix domain socket.
package plugin
