package main

import (
	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/transmission"
)

// definitions returns the definitions of the clients, by name.
func definitions() []client.Definition {
	var defs []client.Definition
	defs = append(defs, transmission.Definitions()...)
	return defs
}
