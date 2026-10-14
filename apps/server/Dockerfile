# syntax=docker/dockerfile:1
#
# The Mavio server with jellyfin-ffmpeg's portable build and Mavio's fork of
# rclone, the versions mise.toml pins. Build from the repository root, for one
# or more platforms:
#
#   docker buildx build -f apps/server/Dockerfile --platform linux/amd64,linux/arm64 .
#
# The server is cross-compiled on the build machine's own platform, so
# multi-platform builds need no emulation.

ARG GO_VERSION=1.27.2
ARG NODE_VERSION=26.11.1
ARG BUN_VERSION=1.4.3
ARG JELLYFIN_FFMPEG_VERSION=8.1.3-1
ARG RCLONE_VERSION=1.75.2-mavio

# The web UI, which the server embeds.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-trixie-slim AS web
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
	npm install -g "$(node -p 'require("./package.json").packageManager')" \
	&& pnpm install --frozen-lockfile --filter @mavio/web... \
	&& pnpm --filter @mavio/web build

# The agent plugin, a TypeScript process plugin compiled by Bun for the target platform.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-trixie-slim AS agent
ARG TARGETARCH BUN_VERSION
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
	npm install -g "$(node -p 'require("./package.json").packageManager')" "bun@${BUN_VERSION}" \
	&& pnpm install --frozen-lockfile --filter @mavio/plugin-agent... \
	&& cd plugins/agent && bun scripts/build.ts linux/$TARGETARCH \
	&& mkdir -p /out/plugins && mv dist/plugin /out/plugins/agent

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-trixie AS server
ARG TARGETOS TARGETARCH VERSION
WORKDIR /src
COPY . .
COPY --from=web /src/apps/server/internal/webui/dist/ apps/server/internal/webui/dist/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
	cd apps/server && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
	-ldflags "-s -w ${VERSION:+-X github.com/mavioai/mavio/apps/server/internal/buildinfo.version=$VERSION}" \
	-o /out/mavio ./cmd/mavio \
	&& cd ../../plugins/dlna && mkdir -p /out/plugins/dlna && cp manifest.json /out/plugins/dlna/ \
	&& CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/plugins/dlna/plugin . \
	&& cd ../indexers && mkdir -p /out/plugins/indexers && cp manifest.json /out/plugins/indexers/ \
	&& CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/plugins/indexers/plugin . \
	&& cd ../download-clients && mkdir -p /out/plugins/download-clients && cp manifest.json /out/plugins/download-clients/ \
	&& CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/plugins/download-clients/plugin . \
	&& cd ../subtitles && mkdir -p /out/plugins/subtitles && cp manifest.json /out/plugins/subtitles/ \
	&& CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/plugins/subtitles/plugin .

# Mavio's fork of rclone, the release mise.toml pins.
FROM --platform=$BUILDPLATFORM debian:trixie-slim AS rclone
ARG TARGETARCH RCLONE_VERSION
# GitHub's SHA-256 digests of the release's Linux builds.
ARG RCLONE_SHA256_amd64=c40541a5fbfb94f93147e94b873b0d7f5c345a446312d10ce99f175566e304d7
ARG RCLONE_SHA256_arm64=6d29cfb1ca73e1b5aaf651efadff4db2b0b842999f6676c481a9d33c6e6265af
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip \
	&& rm -rf /var/lib/apt/lists/*
RUN set -eu; \
	case "$TARGETARCH" in \
		amd64) sum=$RCLONE_SHA256_amd64 ;; \
		arm64) sum=$RCLONE_SHA256_arm64 ;; \
		*) echo "no rclone build for $TARGETARCH" >&2; exit 1 ;; \
	esac; \
	name=rclone-v${RCLONE_VERSION}-linux-${TARGETARCH}; \
	curl -fsSLo /tmp/rclone.zip \
		"https://github.com/mavioai/rclone/releases/download/v${RCLONE_VERSION}/$name.zip"; \
	echo "$sum  /tmp/rclone.zip" | sha256sum -c -; \
	unzip -q /tmp/rclone.zip -d /tmp && mkdir -p /out && mv /tmp/$name/rclone /out/rclone

FROM --platform=$BUILDPLATFORM debian:trixie-slim AS ffmpeg
ARG TARGETARCH JELLYFIN_FFMPEG_VERSION
# GitHub's SHA-256 digests of the release's portable builds.
ARG SHA256_amd64=b86dc023c64a5a9a410e6e3a938a970460214fae78f31c1553c9f88f1c289b6a
ARG SHA256_arm64=7bc8e8d0986f7f4693f63e9728570f671f07b6e21c05d5465dd7ce461d1631dc
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
	&& rm -rf /var/lib/apt/lists/*
RUN set -eu; \
	case "$TARGETARCH" in \
		amd64) asset=linux64; sum=$SHA256_amd64 ;; \
		arm64) asset=linuxarm64; sum=$SHA256_arm64 ;; \
		*) echo "no jellyfin-ffmpeg build for $TARGETARCH" >&2; exit 1 ;; \
	esac; \
	file=jellyfin-ffmpeg_${JELLYFIN_FFMPEG_VERSION}_portable_${asset}-gpl.tar.xz; \
	curl -fsSLo /tmp/ffmpeg.tar.xz \
		"https://github.com/jellyfin/jellyfin-ffmpeg/releases/download/v${JELLYFIN_FFMPEG_VERSION}/$file"; \
	echo "$sum  /tmp/ffmpeg.tar.xz" | sha256sum -c -; \
	mkdir -p /out && tar -xJf /tmp/ffmpeg.tar.xz -C /out ffmpeg ffprobe

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
	&& rm -rf /var/lib/apt/lists/* \
	&& groupadd --system --gid 1000 mavio \
	&& useradd --system --uid 1000 --gid mavio --home-dir /config mavio \
	&& mkdir -p /config /cache /media && chown -R mavio:mavio /config /cache
COPY --from=ffmpeg /out/ /usr/lib/jellyfin-ffmpeg/
COPY --from=server /out/mavio /usr/local/bin/mavio
# Run as the rcd of libraries on rclone remotes; its config is /config/rclone/rclone.conf.
COPY --from=rclone /out/rclone /usr/local/bin/rclone
# The plugins the image ships, installed into /config/plugins on the first start.
COPY --from=server /out/plugins/ /usr/lib/mavio/plugins/
COPY --from=agent /out/plugins/ /usr/lib/mavio/plugins/
USER mavio
# /config is Mavio's home: config.toml, the database, plugins, chosen artwork and backups; /cache
# holds images, compiled plugins and transcodes. Mount media libraries under /media.
ENV MAVIO_HOME=/config MAVIO_CACHE_DIR=/cache MAVIO_PLUGIN_SEED_DIR=/usr/lib/mavio/plugins \
	MAVIO_FFMPEG=/usr/lib/jellyfin-ffmpeg/ffmpeg MAVIO_FFPROBE=/usr/lib/jellyfin-ffmpeg/ffprobe
VOLUME ["/config", "/cache"]
# 1900/udp is DLNA's SSDP; its multicast reaches devices only with the host's network (--network host).
EXPOSE 8686 7359/udp 1900/udp
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s \
	CMD curl -fsS -H 'Content-Type: application/json' -d '{}' \
		http://127.0.0.1:8686/mavio.system.v1.SystemService/GetHealth || exit 1
# Arguments after the image name are flags, e.g. `docker run … mavio:dev --database postgres://…`;
# they and `-e MAVIO_…` variables override the settings above and those of /config/config.toml.
ENTRYPOINT ["mavio"]
