package rpc

import (
	"net/http"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// TestManualImportService checks the errors of the manual import RPCs;
// the acquisition runtime's tests cover the imports.
func TestManualImportService(t *testing.T) {
	ctx := t.Context()
	f := newAcquisitionFixture(t)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		mux := http.NewServeMux()
		mux.Handle(acquisitionv1connect.NewDownloadServiceHandler(NewDownloadService(f.db, f.rt), opts))
		return "/", mux
	})
	admin := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, url, as("admin"))
	user := acquisitionv1connect.NewDownloadServiceClient(http.DefaultClient, url, as("user"))
	unknown := core.NewID().String()

	_, err := user.ListManualImport(ctx, acquisitionv1.ListManualImportRequest_builder{Id: new(unknown)}.Build())
	wantCode(t, "ListManualImport as user", err, connect.CodePermissionDenied)
	_, err = admin.ListManualImport(ctx, acquisitionv1.ListManualImportRequest_builder{Id: new(unknown)}.Build())
	wantCode(t, "ListManualImport of an unknown download", err, connect.CodeNotFound)
	_, err = user.ManualImport(ctx, acquisitionv1.ManualImportRequest_builder{Id: new(unknown), Path: new("/downloads/a.mkv")}.Build())
	wantCode(t, "ManualImport as user", err, connect.CodePermissionDenied)
	_, err = admin.ManualImport(ctx, acquisitionv1.ManualImportRequest_builder{Id: new(unknown), Path: new("/downloads/a.mkv"), Quality: new("Bluray-9000p")}.Build())
	wantCode(t, "ManualImport with an unknown quality", err, connect.CodeInvalidArgument)
	_, err = admin.ManualImport(ctx, acquisitionv1.ManualImportRequest_builder{Id: new(unknown), Path: new("/downloads/a.mkv"), Quality: new("bluray-1080p")}.Build())
	wantCode(t, "ManualImport of an unknown download", err, connect.CodeNotFound)
}

func TestQualityNamed(t *testing.T) {
	for _, tc := range []struct {
		name string
		want release.Quality
		ok   bool
	}{
		{"Bluray-1080p", release.Bluray1080p, true},
		{"remux-2160p", release.Remux2160p, true},
		{"BR-DISK", release.BRDisk, true},
		{"Bluray-9000p", release.Unknown, false},
	} {
		if got, ok := qualityNamed(tc.name); got != tc.want || ok != tc.ok {
			t.Errorf("qualityNamed(%q) = %s, %v, want = %s, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
