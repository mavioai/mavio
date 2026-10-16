// Package intent runs the assistant's grammar (libs/intent), built as one
// WASM module by `nx run intent:wasm` into dist and embedded, on wazero: the
// same module the web UI runs, so that the fast path has one
// implementation. A server built without the module has no fast path of
// its own; Load reports it missing.
package intent
