package plugins

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestTaskProgress(t *testing.T) {
	m := &Manager{}
	job := core.NewID()
	if m.ReportTaskProgress("dev.mavio.voice", "download", 0.5, "") {
		t.Error("ReportTaskProgress of a task not running = true, want false")
	}
	m.runs.start("dev.mavio.voice", "download", job)
	if _, ok := m.JobProgress(job); ok {
		t.Error("JobProgress before a report: ok, want none")
	}
	for _, tt := range []struct{ report, want float64 }{{0.25, 0.25}, {1.5, 1}, {-1, 0}} {
		if !m.ReportTaskProgress("dev.mavio.voice", "download", tt.report, "") {
			t.Fatal("ReportTaskProgress of a running task = false, want true")
		}
		if got, ok := m.JobProgress(job); !ok || got != tt.want {
			t.Errorf("JobProgress after %v = %v, %v, want = %v", tt.report, got, ok, tt.want)
		}
	}
	if m.ReportTaskProgress("dev.mavio.voice", "other", 0.5, "x") {
		t.Error("ReportTaskProgress of another task = true, want false")
	}
	m.ReportTaskProgress("dev.mavio.voice", "download", 0.5, "kokoro · 93/310 MB")
	if got, ok := m.JobStatus(job); !ok || got != "kokoro · 93/310 MB" {
		t.Errorf("JobStatus = %q, %v, want = the status reported", got, ok)
	}
	// An empty status tells nothing.
	m.ReportTaskProgress("dev.mavio.voice", "download", 0.6, "")
	if got, ok := m.JobStatus(job); ok {
		t.Errorf("JobStatus after an empty status = %q, want none", got)
	}
	m.ReportTaskProgress("dev.mavio.voice", "download", 0.7, "piper")
	m.runs.end("dev.mavio.voice", "download")
	if _, ok := m.JobProgress(job); ok {
		t.Error("JobProgress after the run ended: ok, want none")
	}
	if _, ok := m.JobStatus(job); ok {
		t.Error("JobStatus after the run ended: ok, want none")
	}
}
