# Build the manager binary
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache deps so source changes don't invalidate the layer.
RUN go mod download

# Copy the go source
COPY cmd/manager/ cmd/manager/
COPY api/ api/
COPY internal/ internal/

# Build. Leaving GOARCH unset honors the host platform unless overridden.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager ./cmd/manager

# Distroless minimal base for the manager binary.
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
