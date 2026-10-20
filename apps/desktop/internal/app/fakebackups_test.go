package app

import (
	"context"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeBackups serves a fake server's backups, newest first: one made a
// day ago.
type fakeBackups struct {
	systemv1connect.UnimplementedBackupServiceHandler

	mu      sync.Mutex
	backups []*systemv1.Backup
}

func newFakeBackups() *fakeBackups {
	return &fakeBackups{backups: []*systemv1.Backup{systemv1.Backup_builder{
		Name: new("mavio-20261019-120000.zip"), SizeBytes: new(int64(3e6)),
		CreateTime: timestamppb.New(time.Now().Add(-26 * time.Hour)), Version: new("0.9.0"),
	}.Build()}}
}

func (s *fakeBackups) CreateBackup(context.Context, *systemv1.CreateBackupRequest) (*systemv1.CreateBackupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := systemv1.Backup_builder{
		Name: new("mavio-20261020-090000.zip"), SizeBytes: new(int64(4e6)), CreateTime: timestamppb.Now(), Version: new("0.9.0"),
	}.Build()
	s.backups = slices.Insert(s.backups, 0, b)
	return systemv1.CreateBackupResponse_builder{Backup: b}.Build(), nil
}

func (s *fakeBackups) ListBackups(context.Context, *systemv1.ListBackupsRequest) (*systemv1.ListBackupsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return systemv1.ListBackupsResponse_builder{Backups: slices.Clone(s.backups)}.Build(), nil
}

func (s *fakeBackups) DeleteBackup(_ context.Context, r *systemv1.DeleteBackupRequest) (*systemv1.DeleteBackupResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.backups, func(b *systemv1.Backup) bool { return b.GetName() == r.GetName() })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	s.backups = slices.Delete(s.backups, i, i+1)
	return &systemv1.DeleteBackupResponse{}, nil
}
