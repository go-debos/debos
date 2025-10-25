# Global ARGs shared by all stages
ARG DEBIAN_FRONTEND=noninteractive
ARG GOPATH=/usr/local/go
ARG GOPROXY

### first stage - builder ###
FROM debian:bookworm-slim AS builder

ARG DEBIAN_FRONTEND
ARG GOPATH
ARG GOPROXY
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

# Optionally add host CA certificates for environments with MITM proxies
# Usage: DOCKER_BUILDKIT=1 docker build --secret id=cacert,src=/etc/ssl/certs/ca-certificates.crt ...
RUN --mount=type=secret,id=cacert,target=/tmp/host-ca-certificates.crt \
    if [ -f /tmp/host-ca-certificates.crt ]; then \
        cp /tmp/host-ca-certificates.crt /usr/local/share/ca-certificates/host-ca-certificates.crt && \
        update-ca-certificates; \
    fi

# Build debos
ARG DEBOS_VER
COPY . $GOPATH/src/github.com/go-debos/debos
WORKDIR $GOPATH/src/github.com/go-debos/debos/cmd/debos
# Set GOPROXY during build if provided (e.g., GOPROXY=direct for CI with MITM proxies)
RUN env ${GOPROXY:+GOPROXY=$GOPROXY} go install -ldflags="-X main.Version=${DEBOS_VER}" ./...

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY docker/get-archlinux-keyring.sh /
RUN /get-archlinux-keyring.sh /arch-keyring

### second stage - runner ###
FROM debian:bookworm-slim AS runner-amd64
RUN apt-get update && \
    apt-get install -y --no-install-recommends initramfs-tools && \
    rm -rf /var/lib/apt/lists/*
RUN rm /etc/kernel/postinst.d/*
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        linux-image-amd64 \
        qemu-system-x86 \
        user-mode-linux && \
    rm -rf /var/lib/apt/lists/*

FROM debian:bookworm-slim AS runner-arm64
RUN apt-get update && \
    apt-get install -y --no-install-recommends initramfs-tools && \
    rm -rf /var/lib/apt/lists/*
RUN rm /etc/kernel/postinst.d/*
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

LABEL org.label-schema.name="debos"
LABEL org.label-schema.description="Debian OS builder"
LABEL org.label-schema.vcs-url="https://github.com/go-debos/debos"
LABEL org.label-schema.docker.cmd='docker run \
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
        mmdebstrap \
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
        qemu-user-static \
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
        arch-install-scripts \
        arch-test && \
    rm -rf /var/lib/apt/lists/*

# debian's qemu-user-static package no longer registers binfmts
# if running inside a virtualmachine; dockerhub builds are inside a vm
RUN for arch in aarch64 alpha arm armeb cris hexagon hppa m68k microblaze mips mips64 mips64el mipsel mipsn32 mipsn32el ppc ppc64 ppc64le riscv32 riscv64 s390x sh4 sh4eb sparc sparc32plus sparc64 xtensa xtensaeb; do \
      update-binfmts --import qemu-$arch; \
    done

COPY --from=builder $GOPATH/bin/debos /usr/local/bin/debos

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY --from=builder /arch-keyring /usr/share/keyrings

ENTRYPOINT ["/usr/local/bin/debos"]
