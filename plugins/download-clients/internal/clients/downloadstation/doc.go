// Package downloadstation is Synology's Download Station, for torrents and
// for NZBs, ported from Radarr's and Sonarr's TorrentDownloadStation and
// UsenetDownloadStation: tasks added and listed over the DSM web API
// (DownloadStation's task API v2, else v1), put in a shared folder by kind,
// their paths resolved through FileStation, and their IDs prefixed with the
// hash of the Diskstation's serial number.
package downloadstation
