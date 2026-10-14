// Package relay splices clients' TLS connections to servers that cannot
// be reached directly (docs/mavio-dev.md §5), without terminating TLS: it
// reads the server name from the ClientHello, checks the ticket in it,
// and joins the connection to one the server opens back to the relay.
package relay
