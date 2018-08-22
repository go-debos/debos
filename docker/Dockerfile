### first stage - builder ###
FROM golang:1.10 as builder

MAINTAINER Maciej Pijanowski <maciej.pijanowski@3mdeb.com>

ENV HOME=/scratch

# install debos build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libglib2.0-dev \
    libostree-dev \
    && rm -rf /var/lib/apt/lists/*

RUN go get -d github.com/go-debos/debos/cmd/debos
WORKDIR /go/src/github.com/go-debos/debos/
RUN GOOS=linux go build -a cmd/debos/debos.go

### second stage - runner ###
FROM debian:stretch-slim as runner

ARG DEBIAN_FRONTEND=noninteractive

# debos runtime dependencies
# ca-certificates is required to validate HTTPS certificates when getting debootstrap release file
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libostree-1-1 \
    ca-certificates \
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
    dbus \
    && rm -rf /var/lib/apt/lists/*

# Bug description: https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=806780
# It was fixed in debootstrap 1.0.96 while Stretch provides 1.0.89. Backports
# provide 1.0.100.
RUN printf "deb http://httpredir.debian.org/debian stretch-backports main \ndeb-src http://httpredir.debian.org/debian stretch-backports main" > /etc/apt/sources.list.d/backports.list && \
    apt-get update && \
    apt-get -t stretch-backports install -y --no-install-recommends \
    debootstrap && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /go/src/github.com/go-debos/debos/debos /usr/bin/debos

WORKDIR /root
