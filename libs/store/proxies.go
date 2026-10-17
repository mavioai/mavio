package store

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/proxy"
)

type proxies struct{ s *Store }

// Proxies returns the server's proxies.
func (s *Store) Proxies() core.ProxyRepository { return proxies{s} }

func (r proxies) List(ctx context.Context) ([]core.Proxy, error) {
	rows, err := r.s.read.Proxy.Query().Order(ent.Asc(proxy.FieldNameKey)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list proxies")
	}
	out := make([]core.Proxy, len(rows))
	for i, p := range rows {
		out[i] = toProxy(p)
	}
	return out, nil
}

func (r proxies) Get(ctx context.Context, id core.ID) (core.Proxy, error) {
	p, err := r.s.read.Proxy.Get(ctx, id)
	if err != nil {
		return core.Proxy{}, mapErr(err, "get proxy")
	}
	return toProxy(p), nil
}

func (r proxies) Put(ctx context.Context, p *core.Proxy) error {
	if p.ID.IsZero() {
		p.ID = core.NewID()
	}
	if err := p.Validate(); err != nil {
		return err
	}
	return r.s.InTx(ctx, func(tx core.Store) error {
		c := tx.(*Store).write
		other, err := c.Proxy.Query().
			Where(proxy.IDNEQ(p.ID), proxy.Or(proxy.NameKey(p.NameKey()), proxy.AddressKey(p.Key()))).
			First(ctx)
		switch {
		case err == nil && other.NameKey == p.NameKey():
			return fmt.Errorf("%w: proxy %q has the same name", core.ErrConflict, other.Name)
		case err == nil:
			return fmt.Errorf("%w: proxy %q has the same address", core.ErrConflict, other.Name)
		case !ent.IsNotFound(err):
			return mapErr(err, "put proxy")
		}
		err = c.Proxy.Create().
			SetID(p.ID).
			SetName(p.Name).
			SetNameKey(p.NameKey()).
			SetKind(string(p.Kind)).
			SetHost(p.Host).
			SetPort(p.Port).
			SetUsername(p.Username).
			SetPassword(p.Password).
			SetAddressKey(p.Key()).
			OnConflictColumns(proxy.FieldID).
			UpdateNewValues().
			Exec(ctx)
		return mapErr(err, "put proxy")
	})
}

func (r proxies) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Proxy.DeleteOneID(id).Exec(ctx), "delete proxy")
}

func toProxy(p *ent.Proxy) core.Proxy {
	return core.Proxy{
		ID: p.ID, Name: p.Name, Kind: core.ProxyKind(p.Kind), Host: p.Host, Port: p.Port,
		Username: p.Username, Password: p.Password,
	}
}
