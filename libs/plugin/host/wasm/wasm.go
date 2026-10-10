// Package wasm runs WebAssembly plugins on wazero.
//
// Each plugin gets its own wazero runtime and a pool of module instances; a
// call takes an instance, delivers the Connect request through the module
// ABI (see internal/abi) and returns it to the pool. An instance whose call
// traps, panics or times out is discarded and replaced, so a misbehaving
// call never affects the host or later calls. The last configuration is
// replayed on every new instance.
package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/mavioai/mavio/libs/plugin/internal/abi"
	"github.com/mavioai/mavio/libs/plugin/internal/clients"
	"github.com/mavioai/mavio/libs/plugin/internal/proc"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

const configurePath = "/mavio.plugin.v1.PluginService/Configure"

// Options configures the WASM runtime.
type Options struct {
	// CacheDir persists compiled modules across restarts; empty keeps them in
	// memory only.
	CacheDir string
	// MemoryLimitPages caps each instance's memory in 64 KiB pages; zero
	// means 4096 (256 MiB).
	MemoryLimitPages uint32
	// CallTimeout bounds each call; zero means 30 s.
	CallTimeout time.Duration
	// Instances is the pool size, i.e. the number of concurrent calls; zero
	// means 2.
	Instances int
	// FetchClient performs the plugin's outbound HTTP requests; nil means a
	// client with a 30 s timeout.
	FetchClient *http.Client
	// HostAPI serves the plugin's requests to the host API (abi.HostName);
	// nil denies them.
	HostAPI http.Handler
	// DataDir is mounted writable at /data; empty mounts nothing.
	DataDir string
	Logger  *slog.Logger
}

// DataMount is where WASM plugins see their data folder.
const DataMount = "/data"

type callTimeoutKey struct{}

// WithCallTimeout returns ctx bounding the WASM calls made with it by d
// instead of Options.CallTimeout.
func WithCallTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, callTimeoutKey{}, d)
}

// Plugin is a running WASM plugin. It implements host.Plugin.
type Plugin struct {
	clients.Set
	manifest *pluginv1.Manifest
	opts     Options
	log      *slog.Logger
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	pool     chan *instance

	cfgMu      sync.Mutex
	config     []byte // encoded Configure request body
	configGen  uint64
	nextModule atomic.Uint64

	fetchMu   sync.Mutex
	fetches   map[uint32][]byte
	fetchNext uint32

	closed atomic.Bool
}

type instance struct {
	mod       api.Module
	configGen uint64
}

// Open compiles the module at path and starts the instance pool.
func Open(ctx context.Context, path string, m *pluginv1.Manifest, opts Options) (*Plugin, error) {
	if opts.MemoryLimitPages == 0 {
		opts.MemoryLimitPages = 4096
	}
	if opts.CallTimeout == 0 {
		opts.CallTimeout = 30 * time.Second
	}
	if opts.Instances == 0 {
		opts.Instances = 2
	}
	if opts.FetchClient == nil {
		opts.FetchClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	bin, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(opts.MemoryLimitPages).
		WithCloseOnContextDone(true)
	if opts.CacheDir != "" {
		cache, err := wazero.NewCompilationCacheWithDir(opts.CacheDir)
		if err != nil {
			return nil, err
		}
		cfg = cfg.WithCompilationCache(cache)
	}
	p := &Plugin{
		manifest: m,
		opts:     opts,
		log:      opts.Logger.With("plugin", m.GetId()),
		runtime:  wazero.NewRuntimeWithConfig(ctx, cfg),
		pool:     make(chan *instance, opts.Instances),
		fetches:  map[uint32][]byte{},
	}
	if err := p.setup(ctx, bin); err != nil {
		_ = p.runtime.Close(ctx)
		return nil, err
	}
	p.Set = clients.New(&http.Client{Transport: p}, "http://plugin", m)
	return p, nil
}

func (p *Plugin) setup(ctx context.Context, bin []byte) error {
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, p.runtime); err != nil {
		return err
	}
	_, err := p.runtime.NewHostModuleBuilder(abi.HostModule).
		NewFunctionBuilder().WithFunc(p.hostFetch).Export(abi.ImportFetch).
		NewFunctionBuilder().WithFunc(p.hostRead).Export(abi.ImportRead).
		Instantiate(ctx)
	if err != nil {
		return err
	}
	if p.compiled, err = p.runtime.CompileModule(ctx, bin); err != nil {
		return fmt.Errorf("compile %s: %w", p.manifest.GetId(), err)
	}
	for _, name := range []string{abi.ExportAlloc, abi.ExportFree, abi.ExportCall, abi.ExportInit} {
		if _, ok := p.compiled.ExportedFunctions()[name]; !ok {
			return fmt.Errorf("%s does not export %s; build it with guest/wasm and -buildmode=c-shared", p.manifest.GetId(), name)
		}
	}
	for range p.opts.Instances {
		inst, err := p.newInstance(ctx)
		if err != nil {
			return err
		}
		p.pool <- inst
	}
	return nil
}

func (p *Plugin) newInstance(ctx context.Context) (*instance, error) {
	fsConfig := wazero.NewFSConfig()
	for _, dir := range p.manifest.GetPermissions().GetReadPaths() {
		fsConfig = fsConfig.WithReadOnlyDirMount(dir, dir)
	}
	cfg := wazero.NewModuleConfig()
	if p.opts.DataDir != "" {
		fsConfig = fsConfig.WithDirMount(p.opts.DataDir, DataMount)
		cfg = cfg.WithEnv(proc.EnvData, DataMount)
	}
	name := fmt.Sprintf("%s#%d", p.manifest.GetId(), p.nextModule.Add(1))
	mod, err := p.runtime.InstantiateModule(ctx, p.compiled, cfg.
		WithName(name).
		WithStartFunctions(abi.ExportInit).
		WithStdout(logWriter{p.log, slog.LevelDebug}).
		WithStderr(logWriter{p.log, slog.LevelInfo}).
		WithSysWalltime().
		WithSysNanotime().
		WithFSConfig(fsConfig))
	if err != nil {
		return nil, fmt.Errorf("instantiate %s: %w", p.manifest.GetId(), err)
	}
	return &instance{mod: mod}, nil
}

// RoundTrip delivers a Connect request to an instance. It implements
// http.RoundTripper for the plugin's Connect clients.
func (p *Plugin) RoundTrip(req *http.Request) (*http.Response, error) {
	if p.closed.Load() {
		return nil, errors.New("plugin closed")
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	ctx := req.Context()
	if req.URL.Path == configurePath {
		return p.configure(ctx, req, body)
	}

	var inst *instance
	select {
	case inst = <-p.pool:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	resp, err := p.callConfigured(ctx, inst, abi.Request{Path: req.URL.Path, Header: req.Header, Body: body})
	if err != nil {
		p.log.WarnContext(ctx, "plugin call failed; replacing instance", "path", req.URL.Path, "err", err)
		p.replace(inst)
		return nil, err
	}
	p.pool <- inst
	return httpResponse(req, resp), nil
}

// configure applies a configuration to one instance; if the plugin accepts
// it, it becomes the current configuration, which other instances apply
// before their next call.
func (p *Plugin) configure(ctx context.Context, req *http.Request, body []byte) (*http.Response, error) {
	var inst *instance
	select {
	case inst = <-p.pool:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	resp, err := p.call(ctx, inst, abi.Request{Path: configurePath, Header: req.Header, Body: body})
	if err != nil {
		p.replace(inst)
		return nil, err
	}
	if resp.Status == http.StatusOK {
		p.cfgMu.Lock()
		p.config = body
		p.configGen++
		inst.configGen = p.configGen
		p.cfgMu.Unlock()
	}
	p.pool <- inst
	return httpResponse(req, resp), nil
}

// callConfigured brings inst up to date with the current configuration, then
// calls.
func (p *Plugin) callConfigured(ctx context.Context, inst *instance, req abi.Request) (abi.Response, error) {
	p.cfgMu.Lock()
	cfg, gen := p.config, p.configGen
	p.cfgMu.Unlock()
	if inst.configGen != gen {
		resp, err := p.call(ctx, inst, abi.Request{
			Path:   configurePath,
			Header: http.Header{"Content-Type": {"application/proto"}, "Connect-Protocol-Version": {"1"}},
			Body:   cfg,
		})
		if err != nil {
			return abi.Response{}, err
		}
		if resp.Status != http.StatusOK {
			return abi.Response{}, fmt.Errorf("replaying configuration: status %d: %s", resp.Status, resp.Body)
		}
		inst.configGen = gen
	}
	return p.call(ctx, inst, req)
}

func (p *Plugin) call(ctx context.Context, inst *instance, req abi.Request) (abi.Response, error) {
	timeout := p.opts.CallTimeout
	if d, ok := ctx.Value(callTimeoutKey{}).(time.Duration); ok {
		timeout = d
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	mod := inst.mod
	in := abi.EncodeRequest(req)
	res, err := mod.ExportedFunction(abi.ExportAlloc).Call(ctx, uint64(len(in)))
	if err != nil {
		return abi.Response{}, err
	}
	reqPtr := uint32(res[0])
	defer func() { _, _ = mod.ExportedFunction(abi.ExportFree).Call(context.WithoutCancel(ctx), uint64(reqPtr)) }()
	if !mod.Memory().Write(reqPtr, in) {
		return abi.Response{}, errors.New("request does not fit in guest memory")
	}
	res, err = mod.ExportedFunction(abi.ExportCall).Call(ctx, uint64(reqPtr), uint64(len(in)))
	if err != nil {
		return abi.Response{}, err
	}
	outPtr, outLen := uint32(res[0]>>32), uint32(res[0])
	out, ok := mod.Memory().Read(outPtr, outLen)
	if !ok {
		return abi.Response{}, errors.New("response outside guest memory")
	}
	out = bytes.Clone(out)
	_, _ = mod.ExportedFunction(abi.ExportFree).Call(ctx, uint64(outPtr))
	return abi.DecodeResponse(out)
}

// replace discards a broken instance and puts a fresh one in the pool,
// retrying with backoff so the pool keeps its size.
func (p *Plugin) replace(inst *instance) {
	ctx := context.Background()
	_ = inst.mod.Close(ctx)
	go func() {
		delay := 100 * time.Millisecond
		for !p.closed.Load() {
			fresh, err := p.newInstance(ctx)
			if err == nil {
				p.pool <- fresh
				return
			}
			p.log.Error("cannot replace plugin instance; retrying", "err", err, "in", delay)
			time.Sleep(delay)
			delay = min(2*delay, 30*time.Second)
		}
	}()
}

// Close releases the runtime and all instances.
func (p *Plugin) Close(ctx context.Context) error {
	if p.closed.Swap(true) {
		return nil
	}
	return p.runtime.Close(ctx)
}

// hostFetch implements the http_fetch import: it performs the request
// described by the JSON at (ptr, n) if the manifest allows the host, stores
// the JSON result and returns (handle << 32 | length).
func (p *Plugin) hostFetch(ctx context.Context, m api.Module, ptr, n uint32) uint64 {
	in, ok := m.Memory().Read(ptr, n)
	var out abi.FetchResponse
	if !ok {
		out.Error = "request outside guest memory"
	} else {
		out = p.fetch(ctx, bytes.Clone(in))
	}
	data, _ := json.Marshal(out)
	p.fetchMu.Lock()
	p.fetchNext++
	handle := p.fetchNext
	p.fetches[handle] = data
	p.fetchMu.Unlock()
	return uint64(handle)<<32 | uint64(len(data))
}

// hostRead implements the http_read import: it copies a stored fetch result
// into guest memory and forgets it.
func (p *Plugin) hostRead(_ context.Context, m api.Module, handle, ptr uint32) {
	p.fetchMu.Lock()
	data := p.fetches[handle]
	delete(p.fetches, handle)
	p.fetchMu.Unlock()
	m.Memory().Write(ptr, data)
}

func (p *Plugin) fetch(ctx context.Context, in []byte) abi.FetchResponse {
	var req abi.FetchRequest
	if err := json.Unmarshal(in, &req); err != nil {
		return abi.FetchResponse{Error: "malformed request: " + err.Error()}
	}
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return abi.FetchResponse{Error: "only http and https URLs are allowed"}
	}
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	if u.Host == abi.HostName {
		return p.serveHost(ctx, method, u, req)
	}
	if !manifest.AllowsHost(p.manifest, u.Host) && !manifest.AllowsHost(p.manifest, u.Hostname()) {
		p.log.WarnContext(ctx, "plugin request denied", "host", u.Host)
		return abi.FetchResponse{Error: "host " + u.Host + " is not in the plugin's permissions"}
	}
	hreq, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(req.Body))
	if err != nil {
		return abi.FetchResponse{Error: err.Error()}
	}
	hreq.Header = req.Header
	resp, err := p.opts.FetchClient.Do(hreq)
	if err != nil {
		return abi.FetchResponse{Error: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return abi.FetchResponse{Error: err.Error()}
	}
	return abi.FetchResponse{Status: resp.StatusCode, Header: resp.Header, Body: body}
}

// serveHost serves a host API request in process. It runs while the calling
// guest holds its instance, so a host call that needs this plugin again uses
// another instance.
func (p *Plugin) serveHost(ctx context.Context, method string, u *url.URL, req abi.FetchRequest) abi.FetchResponse {
	if p.opts.HostAPI == nil {
		return abi.FetchResponse{Error: "the host API is not available"}
	}
	hreq, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(req.Body))
	if err != nil {
		return abi.FetchResponse{Error: err.Error()}
	}
	if req.Header != nil {
		hreq.Header = req.Header
	}
	hreq.RemoteAddr = "plugin:" + p.manifest.GetId()
	w := &recorder{header: http.Header{}}
	p.opts.HostAPI.ServeHTTP(w, hreq)
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.overflow {
		return abi.FetchResponse{Error: "host API response exceeds 64 MiB"}
	}
	return abi.FetchResponse{Status: w.status, Header: w.header, Body: w.body.Bytes()}
}

// recorder collects a host API response, up to 64 MiB of body.
type recorder struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	if r.body.Len()+len(b) > 64<<20 {
		r.overflow = true
		return 0, errors.New("response too large")
	}
	return r.body.Write(b)
}

func (r *recorder) Flush() {}

func httpResponse(req *http.Request, r abi.Response) *http.Response {
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", r.Status, http.StatusText(r.Status)),
		StatusCode:    r.Status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.Header,
		Body:          io.NopCloser(bytes.NewReader(r.Body)),
		ContentLength: int64(len(r.Body)),
		Request:       req,
	}
}

// logWriter forwards guest output to the logger, one record per write.
type logWriter struct {
	log   *slog.Logger
	level slog.Level
}

func (w logWriter) Write(b []byte) (int, error) {
	if msg := strings.TrimRight(string(b), "\n"); msg != "" {
		w.log.Log(context.Background(), w.level, msg)
	}
	return len(b), nil
}
