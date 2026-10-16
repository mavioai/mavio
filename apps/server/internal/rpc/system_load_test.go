package rpc

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/load"
	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

func TestGetSystemLoad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := 0
		svc := &SystemService{Load: load.NewFrom(func(context.Context) load.Reading {
			n++
			return load.Reading{
				Time:   time.Now(),
				CPU:    &load.CPUTimes{User: float64(n), Total: float64(4 * n)},
				Memory: &load.Memory{Total: 1000, Used: 400, Server: 10},
				Disk:   &load.Counters{In: uint64(2000 * n), Out: 0},
			}
		})}
		member := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserMember}})
		if _, err := svc.GetSystemLoad(member, &systemv1.GetSystemLoadRequest{}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("GetSystemLoad() by a member error = %v, want = permission denied", err)
		}
		ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: core.User{Kind: core.UserAdmin}})
		if _, err := svc.GetSystemLoad(ctx, &systemv1.GetSystemLoadRequest{}); err != nil {
			t.Fatalf("GetSystemLoad: %v", err)
		}
		time.Sleep(load.Interval)
		synctest.Wait()
		res, err := svc.GetSystemLoad(ctx, &systemv1.GetSystemLoadRequest{})
		if err != nil {
			t.Fatalf("GetSystemLoad: %v", err)
		}
		if len(res.GetSamples()) != 1 {
			t.Fatalf("samples = %d, want = 1", len(res.GetSamples()))
		}
		s := res.GetSamples()[0]
		if got := s.GetCpu().GetUser(); got != 0.25 {
			t.Errorf("cpu user = %v, want = 0.25", got)
		}
		if got := s.GetMemory().GetUsedBytes(); got != 400 {
			t.Errorf("memory used = %v, want = 400", got)
		}
		if got := s.GetDisk().GetReadRate(); got != 1000 {
			t.Errorf("disk read rate = %v, want = 1000", got)
		}
		if s.HasNetwork() {
			t.Errorf("network = %v, want = unset", s.GetNetwork())
		}
		time.Sleep(load.Idle + 2*load.Interval)
	})
}
