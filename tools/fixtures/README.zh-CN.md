# fixtures

> [English](README.md) | 简体中文

用 `ffmpeg -f lavfi` 确定性地生成测试媒体，并为测试提供样本定位。详见 `docs/testing.zh-CN.md` §3。

```bash
pnpm nx run fixtures:media          # 把缺失或有变化的样本生成到 .fixtures/
go run ./cmd/fixtures -list            # 列出全部样本
go run ./cmd/fixtures -only a.mkv,b.ts # 只生成指定样本
go run ./cmd/fixtures -force           # 全部重新生成
pnpm nx run fixtures:dev-library    # 为开发用播放器生成几分钟的电影与一部剧集
```

`dev-library.sh` 生成 `.fixtures/dev-library/{Movies,Shows}`：浏览器可直接播放、需转封装、带字幕、多音轨或需转码（HEVC HDR10）的电影，以及一部两集的剧集，海报由 NFO 文件指定。`mavio -dev -dev-library .fixtures/dev-library` 会把它们添加为媒体库。

样本目录定义在 `catalog.go` 中；每个 `Spec` 声明 ffmpeg 参数、所需编码器以及辅助输入文件。

在测试中：

```go
path := fixtures.Require(t, "multi_track.mkv") // 样本未生成时跳过测试
```
