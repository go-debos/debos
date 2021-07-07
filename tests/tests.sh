#!/bin/bash
set -e

for dir in simple subdirs separatedirs ; do
    pushd ${dir}
    debos $@ main.yaml
    debos $@ main.yaml < /dev/null
    popd
done
