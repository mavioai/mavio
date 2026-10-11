package sabnzbd

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"
)

// Radarr's and Sonarr's SabnzbdQueueTimeConverterFixture.
func TestQueueTimeConverter(t *testing.T) {
	tests := []struct {
		time string
		want time.Duration
		ok   bool
	}{
		// valid_time_formats_should_be_parsed_correctly
		{"0:0:0", 0, true},
		{"0:1:59", 119 * time.Second, true},
		{"0:59:59", 3599 * time.Second, true},
		{"1:0:0", 3600 * time.Second, true},
		{"1:0:0:1", (24*3600 + 1) * time.Second, true},
		{"40:12:14", (40*3600 + 12*60 + 14) * time.Second, true},
		{"1:16:12:14", (40*3600 + 12*60 + 14) * time.Second, true},
		// should_support_pre_1_1_0rc4_format
		{"40:12:14", 40*time.Hour + 12*time.Minute + 14*time.Second, true},
		// invalid_time_formats_should_throw_an_exception
		{"1", 0, false},
		{"0:1", 0, false},
		{"0:0:0:0:1", 0, false},
		{"a:b:c", 0, false},
	}
	for _, tt := range tests {
		var item queueItem
		err := json.Unmarshal(fmt.Appendf(nil, `{"timeleft":%q}`, tt.time), &item)
		if (err == nil) != tt.ok || time.Duration(item.TimeLeft) != tt.want {
			t.Errorf("timeleft %q = %v, %v, want = %v, ok %v", tt.time, time.Duration(item.TimeLeft), err, tt.want, tt.ok)
		}
	}
}

func TestPriorityConverter(t *testing.T) {
	tests := []struct {
		json string
		want priority
	}{
		{`"Force"`, priorityForce},
		{`"High"`, priorityHigh},
		{`"Low"`, priorityLow},
		{`"Paused"`, priorityPaused},
		{`"Default"`, priorityDefault},
		{`"2"`, priorityForce},
		{`-100`, priorityDefault},
		{`"Repair"`, priorityNormal},
		{`"force"`, priorityNormal}, // Enum.TryParse is case-sensitive
	}
	for _, tt := range tests {
		var item queueItem
		if err := json.Unmarshal(fmt.Appendf(nil, `{"priority":%s}`, tt.json), &item); err != nil || item.Priority != tt.want {
			t.Errorf("priority %s = %v, %v, want = %v", tt.json, item.Priority, err, tt.want)
		}
	}
}

func TestStringArrayConverter(t *testing.T) {
	tests := []struct {
		json string
		want []string
		ok   bool
	}{
		{`["tv","movies"]`, []string{"tv", "movies"}, true},
		{`"tv"`, []string{"tv"}, true},
		{`null`, []string{""}, true},
		{`[]`, []string{}, true},
		{`1`, nil, false},
	}
	for _, tt := range tests {
		var m misc
		err := json.Unmarshal(fmt.Appendf(nil, `{"date_categories":%s}`, tt.json), &m)
		if (err == nil) != tt.ok || (tt.ok && !slices.Equal(m.DateCategories, tt.want)) {
			t.Errorf("date_categories %s = %q, %v, want = %q, ok %v", tt.json, m.DateCategories, err, tt.want, tt.ok)
		}
	}
	var m misc
	if err := json.Unmarshal([]byte(`{}`), &m); err != nil || m.DateCategories != nil {
		t.Errorf("no date_categories = %q, %v, want nil", m.DateCategories, err)
	}
}

func TestLooseTypes(t *testing.T) {
	var m misc
	data := `{"enable_tv_sorting":1,"enable_movie_sorting":"0","enable_date_sorting":true,"pre_check":"1",
		"history_retention":0,"history_retention_number":"5"}`
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	if !m.EnableTVSorting || m.EnableMovieSorting || !m.EnableDateSorting || !m.PreCheck ||
		m.HistoryRetention != "0" || m.HistoryRetentionNumber != 5 {
		t.Errorf("misc = %+v, want the loose values read", m)
	}
	var q queue
	if err := json.Unmarshal([]byte(`{"paused":"True","slots":[{"mb":"1.5","mbleft":2,"status":"downloading"}]}`), &q); err != nil {
		t.Fatal(err)
	}
	if !q.Paused || q.Items[0].Size != 1.5 || q.Items[0].SizeLeft != 2 || q.Items[0].Status != statusDownloading {
		t.Errorf("queue = %+v, want the loose values read", q)
	}
	var h historyItem
	if err := json.Unmarshal([]byte(`{"bytes":"1000","status":"Something"}`), &h); err != nil || h.Size != 1000 || h.Status != statusUnknown {
		t.Errorf("history item = %+v, %v, want 1000 bytes of unknown status", h, err)
	}
}
