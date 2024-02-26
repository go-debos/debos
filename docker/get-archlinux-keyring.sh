#!/bin/bash
LINK=https://gitlab.archlinux.org/archlinux/archlinux-keyring/-/releases/permalink/latest
TARGET="${1:-.}"

RELEASE=$(curl  -s -I -o /dev/null -w '%header{location}\n' \
          ${LINK} \
          | sed 's/.*\///')

echo Arch keyring release ${RELEASE}
echo Installing to ${TARGET}
mkdir -p ${TARGET}
curl -s -L ${LINK}/downloads/archlinux-keyring-${RELEASE}.tar.gz \
        | tar xvz --strip-components=1 -C ${TARGET}
