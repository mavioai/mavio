package client

import (
	"log/slog"
	"net/http"
	"time"
)

// Protocol is how a client downloads.
type Protocol int

// The protocols.
const (
	Torrent Protocol = iota + 1
	Usenet
)

// FieldType is how a setting is entered.
type FieldType int

// The field types.
const (
	FieldText FieldType = iota + 1
	FieldNumber
	FieldCheckbox
	FieldSelect
	FieldMultiSelect
	FieldInfo
	// FieldTag is a list of strings entered freely, as Radarr's
	// FieldType.Tag.
	FieldTag
)

// Field is a setting of a client.
type Field struct {
	// Name is the setting's key in the settings' JSON.
	Name     string
	Label    string
	HelpText string
	Type     FieldType
	// Default is the value when unset; nil for none.
	Default any
	// Options are the choices of selects.
	Options  []Option
	Advanced bool
	// Secret settings, such as passwords, are never shown back.
	Secret bool
}

// Option is a choice of a select.
type Option struct {
	Value any
	Label string
}

// Env is what clients are made with.
type Env struct {
	// HTTP sends the clients' requests.
	HTTP *http.Client
	// Now is the clock.
	Now func() time.Time
	Log *slog.Logger
	// Seeds keeps the seed limits of the client's torrents across
	// restarts; nil keeps them in memory.
	Seeds *ClientSeeds
}

// SeedStore returns env.Seeds, or seed limits kept in memory when nil.
func (e Env) SeedStore() *ClientSeeds {
	if e.Seeds != nil {
		return e.Seeds
	}
	s, _ := NewSeeds("")
	return s.For("")
}

// Definition is a kind of download client.
type Definition struct {
	// ID is stable, such as "qbittorrent".
	ID          string
	Name        string
	Description string
	Protocol    Protocol
	Fields      []Field
	// PrefersTorrentFile is set for clients taking torrent files rather
	// than magnet links, as Radarr's PreferTorrentFile.
	PrefersTorrentFile bool
	// New returns a client of the definition from its settings' JSON.
	New func(settings []byte, env Env) (Client, error)
}
