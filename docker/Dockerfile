# Global ARGs shared by all stages
ARG DEBIAN_FRONTEND=noninteractive
ARG GOPATH=/usr/local/go

### first stage - builder ###
FROM debian:bookworm-slim AS builder

ARG DEBIAN_FRONTEND
ARG GOPATH
ENV GOPATH=${GOPATH}

# install debos build and unit-test dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gcc \
        git \
        golang-go \
        libc6-dev \
        libostree-dev \
        unzip && \
    rm -rf /var/lib/apt/lists/*

# Build debos
COPY . $GOPATH/src/github.com/go-debos/debos
WORKDIR $GOPATH/src/github.com/go-debos/debos/cmd/debos
RUN go install ./...

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY docker/get-archlinux-keyring.sh /
RUN /get-archlinux-keyring.sh /arch-keyring

### second stage - runner ###
FROM debian:bookworm-slim AS runner-amd64
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        linux-image-amd64 \
        qemu-system-x86 \
        user-mode-linux && \
    rm -rf /var/lib/apt/lists/*

FROM debian:bookworm-slim AS runner-arm64
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        linux-image-arm64 \
        qemu-system-arm \
        # fixes: qemu-system-aarch64: failed to find romfile "efi-virtio.rom"
        ipxe-qemu && \
    rm -rf /var/lib/apt/lists/*

FROM runner-${TARGETARCH} AS runner

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
        debian-ports-archive-keyring \
        debootstrap \
        dosfstools \
        e2fsprogs \
        equivs \
        fdisk \
        f2fs-tools \
        git \
        gzip \
        pigz \
        libostree-1-1 \
        libslirp-helper \
        openssh-client \
        parted \
        pkg-config \
        qemu-utils \
        rsync \
        systemd \
        systemd-container \
        systemd-resolved \
        u-boot-tools \
        unzip \
        xfsprogs \
        xz-utils \
        zip \
        zstd \
        makepkg \
        pacman-package-manager \
        arch-install-scripts && \
    rm -rf /var/lib/apt/lists/*

RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/bookworm-backports.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends -t bookworm-backports \
        qemu-user-static && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder $GOPATH/bin/debos /usr/local/bin/debos

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY --from=builder /arch-keyring /usr/share/keyrings

ENTRYPOINT ["/usr/local/bin/debos"]
