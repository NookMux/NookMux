FROM golang:1.26.10-alpine@sha256:5cb67f19c5e7994dcb80c074f7cfca8aeeada45bbdca035f73df269cefdcce61 AS go-builder
ENV GO111MODULE=on CGO_ENABLED=0

ARG COMMIT_HASH=""
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
RUN test -f web/dist/index.html
RUN go build -ldflags "-s -w -X 'github.com/NookMux/NookMux/internal/common.Version=${COMMIT_HASH}'" -o NookMux ./cmd/server

# alpine:3.21 (~3.5MB). Previous runtime used debian:bookworm-slim with libasan8
# (AddressSanitizer runtime, ~100MB+ and adds per-allocation overhead).
# The Go binary is statically linked (CGO_ENABLED=0) and uses the pure-Go
# github.com/glebarez/sqlite driver, so libc/libasan are not needed.
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d

# ca-certificates: TLS root CAs for upstream HTTPS calls
# tzdata: timezone data for correct log timestamps (Go bundles TZDATA but
#         third-party libs and user overrides may reference system zoneinfo)
# wget: used by docker-compose healthcheck; keep it in the runtime image so
#       existing deployments do not become unhealthy after the alpine switch.
RUN apk add --no-cache ca-certificates tzdata wget && update-ca-certificates

COPY --from=go-builder /build/NookMux /NookMux

EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/NookMux"]
