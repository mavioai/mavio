// Package recovery restores the administrator's access from the server's
// command line: a standalone server's administrator gets a new password
// (docs/accounts.md §4.5), and a server in mavio.dev mode is unclaimed for
// another account to claim (§3.2). It works on the database directly,
// with the access to the server's files that this needs.
package recovery
