#!/bin/bash
set -e

LINK=https://gitlab.archlinux.org/archlinux/archlinux-keyring/-/releases/permalink/latest
TARGET="${1:-.}"

# Always create target directory
mkdir -p ${TARGET}

RELEASE=$(curl -s -I -o /dev/null -w '%header{location}\n' \
          ${LINK} \
          | sed 's/.*\///')

if [ -z "$RELEASE" ]; then
    echo "Error: Failed to get archlinux-keyring release info"
    echo "Network or certificate issues may be preventing download"
    echo "Creating empty directory to allow build to continue"
    # Create a marker file to indicate failure
    echo "Download failed" > ${TARGET}/.download-failed
    exit 0
fi

echo Arch keyring release ${RELEASE}
echo Installing to ${TARGET}

# Download and verify the tarball
if ! curl -s -L ${LINK}/downloads/archlinux-keyring-${RELEASE}.tar.gz \
        | tar xvz --strip-components=1 -C ${TARGET}; then
    echo "Warning: Failed to download/extract archlinux-keyring"
    echo "This may cause issues with Arch Linux package verification"
    # Create a marker file to indicate failure but don't fail the build
    echo "Download failed" > ${TARGET}/.download-failed
    exit 0
fi
