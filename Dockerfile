FROM registry.access.redhat.com/ubi8/go-toolset:latest as builder

USER 0
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go


# APP IMAGE 
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

RUN microdnf install --setopt=tsflags=nodocs -y go-toolset && \
    microdnf install -y rsync tar procps-ng && \
    microdnf upgrade -y && \
    microdnf clean all

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65534:65534

ENTRYPOINT ["/manager"]
