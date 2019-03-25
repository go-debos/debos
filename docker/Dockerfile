# Global ARGs shared by all stages
ARG DEBIAN_FRONTEND=noninteractive
ARG GOPATH=/usr/local/go

### first stage - builder ###
FROM debian:buster-slim as builder

ARG DEBIAN_FRONTEND
ARG GOPATH
ENV GOPATH=${GOPATH}

# install debos build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        gcc \
        git \
        golang-go \
        libc6-dev \
        libostree-dev && \
    rm -rf /var/lib/apt/lists/*

# Build debos
COPY . $GOPATH/src/github.com/go-debos/debos
WORKDIR $GOPATH/src/github.com/go-debos/debos/cmd/debos
RUN go get -d ./... && \
    go get -d github.com/stretchr/testify && \
    go install

### second stage - runner ###
FROM debian:buster-slim as runner

ARG DEBIAN_FRONTEND
ARG GOPATH

# Set HOME to a writable directory in case something wants to cache things
ENV HOME=/tmp

LABEL org.label-schema.name "debos"
LABEL org.label-schema.description "Debian OS builder"
LABEL org.label-schema.vcs-url = "https://github.com/go-debos/debos"
LABEL org.label-schema.docker.cmd 'docker run \
  --rm \
  --interactive \
  --tty \
  --device /dev/kvm \
  --user $(id -u) \
  --workdir /recipes \
  --mount "type=bind,source=$(pwd),destination=/recipes" \
  --security-opt label=disable'

# debos runtime dependencies
# ca-certificates is required to validate HTTPS certificates when getting debootstrap release file
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        apt-transport-https \
        binfmt-support \
        bmap-tools \
        btrfs-progs \
        busybox \
        bzip2 \
        ca-certificates \
        debootstrap \
        dosfstools \
        e2fsprogs \
        gzip \
        libostree-1-1 \
        linux-image-amd64 \
        parted \
        pkg-config \
        qemu-system-x86 \
        qemu-user-static \
        systemd \
        systemd-container \
        xz-utils && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder $GOPATH/bin/debos /usr/local/bin/debos

ENTRYPOINT ["/usr/local/bin/debos"]
