package rpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

var libraryKinds = map[core.LibraryKind]libraryv1.LibraryKind{
	core.LibraryMovies:      libraryv1.LibraryKind_LIBRARY_KIND_MOVIES,
	core.LibraryShows:       libraryv1.LibraryKind_LIBRARY_KIND_SHOWS,
	core.LibraryMusic:       libraryv1.LibraryKind_LIBRARY_KIND_MUSIC,
	core.LibraryMusicVideos: libraryv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS,
	core.LibraryHomeVideos:  libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS,
	core.LibraryBooks:       libraryv1.LibraryKind_LIBRARY_KIND_BOOKS,
	core.LibraryPhotos:      libraryv1.LibraryKind_LIBRARY_KIND_PHOTOS,
	core.LibraryMixed:       libraryv1.LibraryKind_LIBRARY_KIND_MIXED,
}

// LibraryService implements mavio.library.v1.LibraryService. Users see the
// libraries their policy allows; only administrators see folder paths and
// manage libraries.
type LibraryService struct {
	store core.Store
	now   func() time.Time
}

var _ libraryv1connect.LibraryServiceHandler = (*LibraryService)(nil)

// NewLibraryService returns a LibraryService backed by store.
func NewLibraryService(store core.Store) *LibraryService {
	return &LibraryService{store: store, now: time.Now}
}

// ListLibraries lists the libraries the caller may access.
func (s *LibraryService) ListLibraries(ctx context.Context, _ *libraryv1.ListLibrariesRequest) (*libraryv1.ListLibrariesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := s.store.Libraries().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	var out []*libraryv1.Library
	for i := range libs {
		if p.User.Policy.CanAccessLibrary(libs[i].ID) {
			out = append(out, libraryToProto(&libs[i], p.User.Admin))
		}
	}
	return libraryv1.ListLibrariesResponse_builder{Libraries: out}.Build(), nil
}

// GetLibrary returns a library the caller may access.
func (s *LibraryService) GetLibrary(ctx context.Context, req *libraryv1.GetLibraryRequest) (*libraryv1.GetLibraryResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	id := core.MustParseID(req.GetId())
	if !p.User.Policy.CanAccessLibrary(id) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such library"))
	}
	lib, err := s.store.Libraries().Get(ctx, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.GetLibraryResponse_builder{Library: libraryToProto(&lib, p.User.Admin)}.Build(), nil
}

// CreateLibrary creates a library and queues its first scan.
func (s *LibraryService) CreateLibrary(ctx context.Context, req *libraryv1.CreateLibraryRequest) (*libraryv1.CreateLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	lib, err := libraryFromSpec(req.GetSpec())
	if err != nil {
		return nil, err
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Libraries().Create(ctx, &lib); err != nil {
			return err
		}
		return s.queueScans(ctx, tx, &lib)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.CreateLibraryResponse_builder{Library: libraryToProto(&lib, true)}.Build(), nil
}

// UpdateLibrary changes a library and queues a scan of it.
func (s *LibraryService) UpdateLibrary(ctx context.Context, req *libraryv1.UpdateLibraryRequest) (*libraryv1.UpdateLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	spec, err := libraryFromSpec(req.GetSpec())
	if err != nil {
		return nil, err
	}
	var lib core.Library
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if lib, err = tx.Libraries().Get(ctx, core.MustParseID(req.GetId())); err != nil {
			return err
		}
		lib.Name, lib.Kind, lib.Paths, lib.ScanInterval = spec.Name, spec.Kind, spec.Paths, spec.ScanInterval
		lib.PreferredLanguage, lib.MetadataCountry = spec.PreferredLanguage, spec.MetadataCountry
		if err := tx.Libraries().Update(ctx, &lib); err != nil {
			return err
		}
		return s.queueScans(ctx, tx, &lib)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.UpdateLibraryResponse_builder{Library: libraryToProto(&lib, true)}.Build(), nil
}

// queueScans queues a scan of the library now and, with a scan interval,
// the scheduled scan after it.
func (s *LibraryService) queueScans(ctx context.Context, tx core.Store, lib *core.Library) error {
	now := s.now()
	jobs := []core.Job{library.ScanJob(*lib, now, false)}
	if lib.ScanInterval > 0 {
		jobs = append(jobs, library.ScanJob(*lib, now.Add(lib.ScanInterval), true))
	}
	for i := range jobs {
		if _, err := tx.Jobs().Enqueue(ctx, &jobs[i]); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLibrary removes a library and its items; media files stay.
func (s *LibraryService) DeleteLibrary(ctx context.Context, req *libraryv1.DeleteLibraryRequest) (*libraryv1.DeleteLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.store.Libraries().Delete(ctx, core.MustParseID(req.GetId())); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DeleteLibraryResponse{}, nil
}

// ScanLibrary queues a scan unless one is pending or running.
func (s *LibraryService) ScanLibrary(ctx context.Context, req *libraryv1.ScanLibraryRequest) (*libraryv1.ScanLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	lib, err := s.store.Libraries().Get(ctx, core.MustParseID(req.GetId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	job := library.ScanJob(lib, s.now(), false)
	added, err := s.store.Jobs().Enqueue(ctx, &job)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	b := libraryv1.ScanLibraryResponse_builder{Enqueued: &added}
	if added {
		b.JobId = new(job.ID.String())
	}
	return b.Build(), nil
}

// libraryFromSpec converts a validated spec, checking that its paths are
// absolute folders.
func libraryFromSpec(spec *libraryv1.LibrarySpec) (core.Library, error) {
	lib := core.Library{
		Name: spec.GetName(), Paths: spec.GetPaths(),
		PreferredLanguage: spec.GetPreferredLanguage(), MetadataCountry: spec.GetMetadataCountry(),
	}
	for k, v := range libraryKinds {
		if v == spec.GetKind() {
			lib.Kind = k
		}
	}
	if spec.HasScanInterval() {
		lib.ScanInterval = spec.GetScanInterval().AsDuration()
	}
	for _, p := range lib.Paths {
		if !filepath.IsAbs(p) {
			return lib, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("library path %q is not absolute", p))
		}
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			return lib, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("library path %q is not a folder", p))
		}
	}
	return lib, nil
}

// libraryToProto converts a library; folder paths only for administrators.
func libraryToProto(lib *core.Library, admin bool) *libraryv1.Library {
	b := libraryv1.Library_builder{
		Id:                new(lib.ID.String()),
		Name:              &lib.Name,
		Kind:              new(libraryKinds[lib.Kind]),
		PreferredLanguage: &lib.PreferredLanguage,
		MetadataCountry:   &lib.MetadataCountry,
		CreateTime:        timestamppb.New(lib.CreatedAt),
		UpdateTime:        timestamppb.New(lib.UpdatedAt),
	}
	if admin {
		b.Paths = lib.Paths
	}
	if lib.ScanInterval > 0 {
		b.ScanInterval = durationpb.New(lib.ScanInterval)
	}
	return b.Build()
}
