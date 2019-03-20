# Global ARGs shared by all stages
ARG DEBIAN_FRONTEND=noninteractive
ARG GOPATH=/usr/local/go

### first stage - builder ###
FROM debian:buster-slim as builder

ARG DEBIAN_FRONTEND
ARG GOPATH

# install debos build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        golang-go \
        gcc \
        git \
        libostree-dev \
        libc6-dev && \
    rm -rf /var/lib/apt/lists/*

# Build debos
COPY . $GOPATH/src/github.com/go-debos/debos
WORKDIR $GOPATH/src/github.com/go-debos/debos/cmd/debos
RUN go get -d ./... && \
    go install

### second stage - runner ###
FROM debian:buster-slim as runner

ARG DEBIAN_FRONTEND
ARG GOPATH

# Set HOME to a writable directory in case something wants to cache things
ENV HOME=/tmp

# debos runtime dependencies
# ca-certificates is required to validate HTTPS certificates when getting debootstrap release file
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libostree-1-1 \
        apt-transport-https \
        ca-certificates \
        debootstrap \
        pkg-config \
        systemd-container \
        binfmt-support \
        parted \
        dosfstools \
        e2fsprogs \
        btrfs-progs \
        bmap-tools \
        # fakemachine runtime dependencies
        qemu-system-x86 \
        qemu-user-static \
        busybox \
        linux-image-amd64 \
        systemd && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder $GOPATH/bin/debos /usr/local/bin/debos