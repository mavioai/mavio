// Package capi is the Go binding of the shared core's C ABI, which the
// desktop app calls (docs/clients.md §5.3). It loads the core's
// shared library at run time, without cgo, and exchanges the messages of
// mavio.capi.v1 (package capiv1) with it: each call is a Request answered
// with a Message through the one callback the binding creates, which also
// brings the servers' events.
package capi
