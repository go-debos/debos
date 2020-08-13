#!/bin/bash
set -e

for dir in simple subdirs separatedirs ; do
    pushd ${dir}
    debos --disable-fakemachine main.yaml
    popd
done
