# syntax=docker/dockerfile:1

# --- build stage -------------------------------------------------------------
# Pinned to a digest-free but explicit minor version: reproducible enough for a
# lab, still easy to bump.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Copy the manifests first and download modules as their own layer. Source
# changes then do not invalidate the (slow) dependency download -- this is the
# layer-caching lesson from lab 1 applied to a real build.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary, which is what lets the final stage be
# a scratch-like image with no libc.
# -trimpath strips local filesystem paths from the binary; -s -w drop the debug
# symbol and DWARF tables, cutting the binary size roughly in half.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api .

# --- runtime stage -----------------------------------------------------------
# distroless static: no shell, no package manager, no setuid binaries. Nothing
# for an attacker who gets RCE to pivot with. The cost is that you cannot
# `kubectl exec` into a shell -- which is the trade-off worth noticing.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /app/api

# Numeric UID, not a name: Kubernetes' runAsNonRoot check reads the numeric id
# from the image config and cannot resolve a username against /etc/passwd.
USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/api"]
