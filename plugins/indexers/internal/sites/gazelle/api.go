package gazelle

import "github.com/mavioai/mavio/plugins/indexers/internal/jsonx"

// response is a Gazelle API response.
type response[T any] struct {
	Status   jsonx.String `json:"status"`
	Response *struct {
		Results []T `json:"results"`
	} `json:"response"`
}

// errorResponse is a Gazelle API error.
type errorResponse struct {
	Error string `json:"error"`
}

// group is a release group of a browse response, with its torrents or,
// for sites listing torrents alone, those of its one torrent.
type group struct {
	GroupID   jsonx.String `json:"groupId"`
	GroupName jsonx.String `json:"groupName"`
	Artist    jsonx.String `json:"artist"`
	GroupYear jsonx.String `json:"groupYear"`
	GroupTime jsonx.String `json:"groupTime"`
	Cover     jsonx.String `json:"cover"`
	// ReleaseType is the group's kind of release, such as "Album".
	ReleaseType jsonx.String  `json:"releaseType"`
	Category    *jsonx.String `json:"category"`
	Torrents    []torrent     `json:"torrents"`
	torrentStats
}

// torrent is a torrent of a release group.
type torrent struct {
	RemasterTitle jsonx.String  `json:"remasterTitle"`
	RemasterYear  jsonx.String  `json:"remasterYear"`
	HasLog        jsonx.Bool    `json:"hasLog"`
	LogScore      jsonx.Int     `json:"logScore"`
	ReleaseName   jsonx.String  `json:"releaseName"`
	Media         jsonx.String  `json:"media"`
	Encoding      jsonx.String  `json:"encoding"`
	Format        jsonx.String  `json:"format"`
	HasCue        jsonx.Bool    `json:"hasCue"`
	Scene         jsonx.Bool    `json:"scene"`
	Time          jsonx.Time    `json:"time"`
	Category      *jsonx.String `json:"category"`
	torrentStats
}

// torrentStats are what torrents and groups listed as torrents have in
// common.
type torrentStats struct {
	TorrentID           jsonx.Int    `json:"torrentId"`
	Size                jsonx.String `json:"size"`
	FileCount           jsonx.Int    `json:"fileCount"`
	Snatches            *jsonx.Int   `json:"snatches"`
	Seeders             jsonx.String `json:"seeders"`
	Leechers            jsonx.String `json:"leechers"`
	IsFreeLeech         jsonx.Bool   `json:"isFreeLeech"`
	IsNeutralLeech      jsonx.Bool   `json:"isNeutralLeech"`
	IsFreeload          jsonx.Bool   `json:"isFreeload"`
	IsPersonalFreeLeech jsonx.Bool   `json:"isPersonalFreeLeech"`
	CanUseToken         jsonx.Bool   `json:"canUseToken"`
}
