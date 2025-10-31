# syntax=docker/dockerfile:1.6

## Build stage ##############################################################
FROM golang:1.22-bookworm AS builder

WORKDIR /workspace

# Copy go module files first to leverage Docker layer caching for dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy the remainder of the source tree.
COPY . .

# Compile the service for the target platform. TARGETOS/TARGETARCH are injected
# automatically when using `docker buildx build --platform=...` (e.g. for K8s).
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" \
    -o /workspace/bin/scte224service ./cmd/scte224service


## Runtime stage ############################################################
FROM gcr.io/distroless/base-debian12 AS runtime

WORKDIR /srv/app

# Copy the compiled binary from the builder stage.
COPY --from=builder /workspace/bin/scte224service /srv/app/scte224service

# Distroless already provides the nonroot user (UID/GID 65532).
USER 65532:65532

ENV HTTP_ADDRESS=:8080 \
    TEMPORAL_HOST_PORT=temporal-frontend:7233 \
    TEMPORAL_NAMESPACE=default \
    TEMPORAL_TASK_QUEUE=scte224-scheduler

EXPOSE 8080

ENTRYPOINT ["/srv/app/scte224service"]
