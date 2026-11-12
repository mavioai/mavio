# syntax=docker/dockerfile:1
#
# apps/dns on distroless, built from its own directory for one or more platforms:
#
#   docker buildx build --platform linux/amd64,linux/arm64 apps/dns
#
# It is cross-compiled on the build machine's own platform, so multi-platform
# builds need no emulation.

ARG GO_VERSION=1.27.2

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/mavio-dns .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/mavio-dns /usr/local/bin/mavio-dns
ENTRYPOINT ["/usr/local/bin/mavio-dns"]
