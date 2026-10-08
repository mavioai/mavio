# fixtures

> English | [简体中文](README.zh-CN.md)

Deterministically generates test media with `ffmpeg -f lavfi` and lets tests locate it. See `docs/testing.md` §3.

```bash
pnpm nx run fixtures:generate          # generate missing or changed fixtures into .fixtures/
go run ./cmd/fixtures -list            # list the catalog
go run ./cmd/fixtures -only a.mkv,b.ts # generate selected fixtures
go run ./cmd/fixtures -force           # regenerate everything
```

The catalog lives in `catalog.go`; each `Spec` declares the ffmpeg arguments, the encoders it needs and any auxiliary input files.

In tests:

```go
path := fixtures.Require(t, "multi_track.mkv") // skips the test if not generated
```
