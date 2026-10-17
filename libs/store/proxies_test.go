package store_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestProxies(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.Proxies()
		b := core.Proxy{Name: "Home", Kind: core.ProxySOCKS5, Host: "10.0.0.2", Port: 1080, Username: "u", Password: "secret"}
		a := core.Proxy{ID: core.NewID(), Name: "away", Kind: core.ProxyHTTP, Host: "proxy.example.com", Port: 8080}
		for _, p := range []*core.Proxy{&b, &a} {
			if err := repo.Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		list, err := repo.List(ctx)
		if err != nil || !reflect.DeepEqual(list, []core.Proxy{a, b}) {
			t.Errorf("List = %+v, %v; want = away, Home", list, err)
		}
		if got, err := repo.Get(ctx, b.ID); err != nil || got != b {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, b)
		}
		for _, tc := range []struct {
			name string
			p    core.Proxy
			want string
		}{
			{"same name", core.Proxy{Name: "HOME", Kind: core.ProxyHTTP, Host: "h", Port: 1}, `"Home" has the same name`},
			{"same address of another kind", core.Proxy{Name: "x", Kind: core.ProxyHTTP, Host: "Proxy.Example.com.", Port: 8080}, `"away" has the same address`},
		} {
			if err := repo.Put(ctx, &tc.p); !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Put of %s: %v, want ErrConflict with %s", tc.name, err, tc.want)
			}
		}
		other := core.Proxy{Name: "home 2", Kind: core.ProxySOCKS5, Host: "10.0.0.2", Port: 1080, Username: "v"}
		if err := repo.Put(ctx, &other); err != nil {
			t.Errorf("Put of another username: %v", err)
		}
		b.Name, b.Port = "home", 1081
		if err := repo.Put(ctx, &b); err != nil {
			t.Fatalf("Put of an update: %v", err)
		}
		if got, _ := repo.Get(ctx, b.ID); got != b {
			t.Errorf("Get after update = %+v, want = %+v", got, b)
		}
		if err := repo.Put(ctx, &core.Proxy{Name: "bad", Kind: core.ProxyHTTP, Host: "h", Port: 0}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put of port 0: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
		if _, err := repo.Get(ctx, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted proxy: %v, want ErrNotFound", err)
		}
	})
}
