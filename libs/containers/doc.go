// Package containers reaches a container engine through the Docker Engine
// API: it finds the engine's socket, named pipe or loopback address,
// negotiates the API version, and creates, starts, stops, inspects and
// removes the images, containers, networks and volumes the server runs
// plugins' container dependencies with. Podman's Docker-compatible API is
// the same API.
//
// It speaks the API with net/http and links no Docker SDK. What it lets
// callers create is deliberately narrow: containers take named volumes,
// one TCP port and NVIDIA GPUs, never privileged mode, added capabilities,
// devices, host namespaces or bind mounts of host paths.
//
// The package depends only on libs/core and the standard library.
package containers
