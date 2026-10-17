// Package dependencies keeps plugins' container dependencies: it finds the
// container engine, combines what plugins' manifests declare with what
// administrators chose, generates the dependencies' secrets and tells
// whether each can be used.
package dependencies
