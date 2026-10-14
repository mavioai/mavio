// Package identity keeps the server's Ed25519 identity key, created on
// first start and kept in the server's home, outside its database, so
// that a copy of the database signs nothing (docs/accounts.md §5.1).
package identity
