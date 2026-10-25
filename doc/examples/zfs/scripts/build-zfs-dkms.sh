#!/bin/bash

set -e

KVER=$(ls /lib/modules)
ZVER=$(ls -d /usr/src/zfs-*); ZVER=${ZVER##*-}

dkms add "zfs/$ZVER"
dkms install -k "$KVER" "zfs/$ZVER"

update-initramfs -u -k "$KVER"
