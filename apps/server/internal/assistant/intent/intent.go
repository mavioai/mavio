package intent

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/protobuf/proto"

	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
)

//go:embed all:dist
var dist embed.FS

// ErrMissing is a server built without the module.
var ErrMissing = errors.New("the assistant's grammar was not built into the server")

// Grammar runs the module. Its methods are safe for concurrent use: each
// call takes an instance of its own, made when none is free.
type Grammar struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule

	mu   sync.Mutex
	free []*instance
}

type instance struct {
	mod                api.Module
	alloc, dealloc     api.Function
	parse, reply, exam api.Function
	languages          api.Function
}

// Load compiles the embedded module.
func Load(ctx context.Context) (*Grammar, error) {
	b, err := fs.ReadFile(dist, "dist/intent.wasm")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, err
	}
	return New(ctx, b)
}

// New compiles a module.
func New(ctx context.Context, wasm []byte) (*Grammar, error) {
	r := wazero.NewRuntime(ctx)
	c, err := r.CompileModule(ctx, wasm)
	if err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("compile the grammar: %w", err)
	}
	return &Grammar{runtime: r, compiled: c}, nil
}

// Close releases the module.
func (g *Grammar) Close(ctx context.Context) error { return g.runtime.Close(ctx) }

func (g *Grammar) take(ctx context.Context) (*instance, error) {
	g.mu.Lock()
	if n := len(g.free); n > 0 {
		in := g.free[n-1]
		g.free = g.free[:n-1]
		g.mu.Unlock()
		return in, nil
	}
	g.mu.Unlock()
	// Instances are anonymous, so that several coexist.
	mod, err := g.runtime.InstantiateModule(ctx, g.compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, fmt.Errorf("instantiate the grammar: %w", err)
	}
	in := &instance{
		mod: mod, alloc: mod.ExportedFunction("intent_alloc"), dealloc: mod.ExportedFunction("intent_free"),
		parse: mod.ExportedFunction("intent_parse"), reply: mod.ExportedFunction("intent_reply"),
		exam: mod.ExportedFunction("intent_examples"), languages: mod.ExportedFunction("intent_languages"),
	}
	if in.alloc == nil || in.dealloc == nil || in.parse == nil || in.reply == nil || in.exam == nil || in.languages == nil {
		_ = mod.Close(ctx)
		return nil, errors.New("the grammar module lacks an export")
	}
	return in, nil
}

func (g *Grammar) give(in *instance) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.free = append(g.free, in)
}

// call passes req to the function pick returns and decodes the answer
// into resp; req nil calls a function of no arguments.
func (g *Grammar) call(ctx context.Context, pick func(*instance) api.Function, req, resp proto.Message) (err error) {
	in, err := g.take(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// An instance that failed may be broken; it is dropped.
		if err != nil {
			_ = in.mod.Close(ctx)
			return
		}
		g.give(in)
	}()
	var args []uint64
	if req != nil {
		b, err := proto.Marshal(req)
		if err != nil {
			return err
		}
		res, err := in.alloc.Call(ctx, uint64(len(b)))
		if err != nil {
			return fmt.Errorf("grammar: %w", err)
		}
		ptr := uint32(res[0])
		defer func() { _, _ = in.dealloc.Call(ctx, uint64(ptr), uint64(len(b))) }()
		if !in.mod.Memory().Write(ptr, b) {
			return errors.New("grammar: request outside memory")
		}
		args = []uint64{uint64(ptr), uint64(len(b))}
	}
	res, err := pick(in).Call(ctx, args...)
	if err != nil {
		return fmt.Errorf("grammar: %w", err)
	}
	if res[0] == 0 {
		return errors.New("grammar: the request could not be read")
	}
	ptr, n := uint32(res[0]>>32), uint32(res[0])
	defer func() { _, _ = in.dealloc.Call(ctx, uint64(ptr), uint64(n)) }()
	out, ok := in.mod.Memory().Read(ptr, n)
	if !ok {
		return errors.New("grammar: answer outside memory")
	}
	return proto.Unmarshal(out, resp)
}

// Parse tells what a request means.
func (g *Grammar) Parse(ctx context.Context, req *intentv1.ParseIntentRequest) (*intentv1.ParseIntentResponse, error) {
	resp := &intentv1.ParseIntentResponse{}
	return resp, g.call(ctx, func(in *instance) api.Function { return in.parse }, req, resp)
}

// Reply writes an answer's sentence.
func (g *Grammar) Reply(ctx context.Context, req *intentv1.ReplyRequest) (*intentv1.ReplyResponse, error) {
	resp := &intentv1.ReplyResponse{}
	return resp, g.call(ctx, func(in *instance) api.Function { return in.reply }, req, resp)
}

// Examples returns examples of requests in a language.
func (g *Grammar) Examples(ctx context.Context, req *intentv1.ExamplesRequest) (*intentv1.ExamplesResponse, error) {
	resp := &intentv1.ExamplesResponse{}
	return resp, g.call(ctx, func(in *instance) api.Function { return in.exam }, req, resp)
}

// Languages returns the languages the grammars understand.
func (g *Grammar) Languages(ctx context.Context) (*intentv1.LanguagesResponse, error) {
	resp := &intentv1.LanguagesResponse{}
	return resp, g.call(ctx, func(in *instance) api.Function { return in.languages }, nil, resp)
}
