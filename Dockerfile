# Global ARGs shared by all stages
ARG DEBIAN_FRONTEND=noninteractive
ARG GOPATH=/usr/local/go

### first stage - builder ###
FROM debian:trixie-slim AS builder

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
RUN go install -ldflags="-X main.Version=${DEBOS_VER}" ./...

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY docker/get-archlinux-keyring.sh /
RUN /get-archlinux-keyring.sh /arch-keyring

### second stage - runner ###
# Install initramfs-tools and drop the kernel postinst hooks before installing
# the kernel, so installing linux-image doesn't trigger an initramfs rebuild.
FROM debian:trixie-slim AS runner-base
ARG DEBIAN_FRONTEND
RUN apt-get update && \
    apt-get install -y --no-install-recommends initramfs-tools && \
    rm -rf /var/lib/apt/lists/* && \
    rm /etc/kernel/postinst.d/*

FROM runner-base AS runner-amd64
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        linux-image-amd64 \
        qemu-system-x86 && \
    rm -rf /var/lib/apt/lists/*

FROM runner-base AS runner-arm64
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
        binfmt-support \
        btrfs-progs \
        busybox \
        ca-certificates \
        debian-ports-archive-keyring \
        debootstrap \
        dosfstools \
        e2fsprogs \
        f2fs-tools \
        fdisk \
        libostree-1-1 \
        mmdebstrap \
        parted \
        pkg-config \
        qemu-user-binfmt \
        qemu-utils \
        systemd \
        systemd-container \
        systemd-resolved \
        xfsprogs \
        makepkg \
        pacman-package-manager \
        arch-install-scripts \
        arch-test && \
    rm -rf /var/lib/apt/lists/*

# Convenience tools commonly used in recipes & test recipes
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        android-sdk-libsparse-utils \
        bmaptool \
        bzip2 \
        devscripts \
        equivs \
        git \
        gzip \
        jq \
        openssh-client \
        pigz \
        rsync \
        u-boot-tools \
        unzip \
        wget \
        xz-utils \
        zip \
        zstd && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder $GOPATH/bin/debos /usr/local/bin/debos

# Install the latest archlinux-keyring, since the one in Debian is bound
# to get outdated sooner or later.
# WARNING: returning to the debian package will break the pacstrap action
COPY --from=builder /arch-keyring /usr/share/keyrings

ENTRYPOINT ["/usr/local/bin/debos"]
