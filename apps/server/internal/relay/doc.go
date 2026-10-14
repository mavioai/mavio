// Package relay keeps a claimed server reachable through mavio.dev's
// relay (docs/mavio-dev.md §5): it holds a control connection to the
// nearest relay, records its region at mavio.dev so clients are given
// tickets for it, and opens a connection back for each client the relay
// announces, which the server serves TLS on with its mavio.site
// certificate.
package relay
