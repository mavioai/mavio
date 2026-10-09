# syntax=docker/dockerfile:1
#
# The Mavio server with jellyfin-ffmpeg's portable build, the one mise.toml
# pins. Build from the repository root, for one or more platforms:
#
#   docker buildx build -f apps/server/Dockerfile --platform linux/amd64,linux/arm64 .
#
# The server is cross-compiled on the build machine's own platform, so
# multi-platform builds need no emulation.

ARG GO_VERSION=1.27.1
ARG JELLYFIN_FFMPEG_VERSION=8.1.3-1

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-trixie AS server
ARG TARGETOS TARGETARCH VERSION
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
	cd apps/server && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
	-ldflags "-s -w ${VERSION:+-X github.com/mavioai/mavio/apps/server/internal/buildinfo.version=$VERSION}" \
	-o /out/mavio ./cmd/mavio

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
	&& mkdir -p /config/plugins /cache /media && chown -R mavio:mavio /config /cache
COPY --from=ffmpeg /out/ /usr/lib/jellyfin-ffmpeg/
COPY --from=server /out/mavio /usr/local/bin/mavio
USER mavio
# /config holds the database, plugins, chosen artwork and backups, /cache images, compiled plugins
# and transcodes; mount media libraries under /media.
VOLUME ["/config", "/cache"]
EXPOSE 8686 7359/udp
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s \
	CMD curl -fsS -H 'Content-Type: application/json' -d '{}' \
		http://127.0.0.1:8686/mavio.system.v1.SystemService/GetHealth || exit 1
# Arguments after the image name are appended, and later flags override
# these, e.g. `docker run … mavio:dev -database postgres://…`.
ENTRYPOINT ["mavio", "-addr", ":8686", "-database", "sqlite:/config/mavio.db", "-plugin-dir", "/config/plugins", \
	"-metadata-dir", "/config/metadata", "-backup-dir", "/config/backups", \
	"-cache-dir", "/cache", "-transcode-dir", "/cache/transcodes", \
	"-ffmpeg", "/usr/lib/jellyfin-ffmpeg/ffmpeg", "-ffprobe", "/usr/lib/jellyfin-ffmpeg/ffprobe"]
