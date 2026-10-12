package rclone

import (
	"context"
	"time"
)

// OfflineStatus is the state of an offline download.
type OfflineStatus string

// The states of offline downloads.
const (
	OfflineInit     OfflineStatus = "init"
	OfflineRunning  OfflineStatus = "running"
	OfflineComplete OfflineStatus = "complete"
	OfflineError    OfflineStatus = "error"
)

// OfflineTask is a download a cloud drive makes itself, from a URL or a
// magnet link, as mavioai/rclone's operations/listoffline gives it.
type OfflineTask struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	URL    string        `json:"url"`
	Size   int64         `json:"size"`
	Status OfflineStatus `json:"status"`
	// Progress is a percentage.
	Progress float64   `json:"progress"`
	Speed    int64     `json:"speed"`
	Error    string    `json:"error_msg"`
	Created  time.Time `json:"created_at"`
}

// AddOffline has the remote of fsName, a folder of it, download link
// there itself; name, when set, names the download.
func (c *Client) AddOffline(ctx context.Context, fsName, link, name string) (OfflineTask, error) {
	var out struct {
		Task OfflineTask `json:"task"`
	}
	in := map[string]any{"fs": fsName, "url": link}
	if name != "" {
		in["name"] = name
	}
	err := c.Call(ctx, "operations/addurl", in, &out)
	return out.Task, err
}

// OfflineTasks lists the offline downloads of a remote.
func (c *Client) OfflineTasks(ctx context.Context, remote string) ([]OfflineTask, error) {
	var out struct {
		Tasks []OfflineTask `json:"tasks"`
	}
	err := c.Call(ctx, "operations/listoffline", map[string]any{"fs": remote + ":"}, &out)
	return out.Tasks, err
}

// DeleteOffline removes an offline download of a remote.
func (c *Client) DeleteOffline(ctx context.Context, remote, id string) error {
	return c.Call(ctx, "operations/deleteoffline", map[string]any{"fs": remote + ":", "task_id": id}, nil)
}
