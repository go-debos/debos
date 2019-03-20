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
RUN go get -d github.com/go-debos/debos/cmd/debos
WORKDIR $GOPATH/src/github.com/go-debos/debos/
RUN GOOS=linux go build -a cmd/debos/debos.go

### second stage - runner ###
FROM debian:buster-slim as runner

ARG DEBIAN_FRONTEND
ARG GOPATH

# debos runtime dependencies
# ca-certificates is required to validate HTTPS certificates when getting debootstrap release file
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libostree-1-1 \
        ca-certificates \
        debootstrap \
        systemd-container \
        binfmt-support \
        parted \
        dosfstools \
        e2fsprogs \
        bmap-tools \
        # fakemachine runtime dependencies
        qemu-system-x86 \
        qemu-user-static \
        busybox \
        linux-image-amd64 \
        systemd \
        dbus && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder $GOPATH/src/github.com/go-debos/debos/debos /usr/bin/debos

WORKDIR /root
