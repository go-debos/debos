#!/bin/bash

set -eu

ESP="$IMAGEMNTDIR/boot/efi"

mkdir -p "$ESP/EFI/BOOT" "$ESP/EFI/ZBM"

if [ -f "$RECIPEDIR/zfsbootmenu.EFI" ]; then
  cp "$RECIPEDIR/zfsbootmenu.EFI" "$ESP/EFI/BOOT/BOOTX64.EFI"
else
  curl -fL -o "$ESP/EFI/BOOT/BOOTX64.EFI" https://get.zfsbootmenu.org/efi
fi

cp "$ESP/EFI/BOOT/BOOTX64.EFI" "$ESP/EFI/ZBM/VMLINUZ-BACKUP.EFI"

zfs set org.zfsbootmenu:commandline="quiet rw" zpool/ROOT
