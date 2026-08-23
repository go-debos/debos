#!/bin/sh

set -e

echo "root:debos" | chroot "$IMAGEMNTDIR" chpasswd
