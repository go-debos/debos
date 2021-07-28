#!/bin/bash
set -e

for dir in simple subdirs separatedirs image; do
    pushd ${dir}
    debos $@ main.yaml
    debos $@ main.yaml < /dev/null
    popd
done
