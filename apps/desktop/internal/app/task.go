package app

import "context"

// task is a value loaded in the background for a key, which names what it
// loads; it keeps the value it had while it loads again for another key.
type task[T any] struct {
	key     string
	value   T
	err     error
	loaded  bool // a value or an error arrived
	loading bool
}

// load starts loading t for key with fn, unless t is for key already, and
// returns t; fn runs on a goroutine of its own, and its result reaches t on
// the main thread.
func load[T any](a *App, t *task[T], key string, fn func(ctx context.Context) (T, error)) *task[T] {
	if t.key == key {
		return t
	}
	t.key, t.loading = key, true
	go func() {
		v, err := fn(a.ctx)
		a.opts.Update(func() {
			if t.key != key {
				return
			}
			t.value, t.err, t.loaded, t.loading = v, err, true, false
		})
	}()
	return t
}
