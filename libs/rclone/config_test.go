package rclone

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAbout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch in["fs"] {
		case "drive:":
			_, _ = io.WriteString(w, `{"total":1000,"used":250,"free":750,"trashed":5}`)
		case "s3:":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"S3 bucket media doesn't support about","status":500}`)
		default:
			_, _ = io.WriteString(w, `{"used":42}`)
		}
	}))
	defer srv.Close()
	c, err := NewClient(Endpoint{URL: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := c.About(t.Context(), "drive")
	if err != nil || u.Total == nil || *u.Total != 1000 || *u.Used != 250 || *u.Free != 750 {
		t.Errorf("About(drive) = %+v, %v, want = 1000 250 750", u, err)
	}
	if u, err := c.About(t.Context(), "sftp"); err != nil || u.Total != nil || u.Used == nil || *u.Used != 42 {
		t.Errorf("About(sftp) = %+v, %v, want = used 42 alone", u, err)
	}
	if _, err := c.About(t.Context(), "s3"); !errors.Is(err, ErrNoUsage) {
		t.Errorf("About(s3) error = %v, want = %v", err, ErrNoUsage)
	}
}
