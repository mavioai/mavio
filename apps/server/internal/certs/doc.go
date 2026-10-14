// Package certs keeps a server's certificate in mavio.dev mode
// (docs/mavio-dev.md §4.2): the wildcard *.<ID>.mavio.site, obtained
// through ACME's DNS-01 challenge with mavio.dev publishing the TXT
// records, renewed when ACME Renewal Information or two thirds of its
// lifetime say, and served without a restart.
package certs
