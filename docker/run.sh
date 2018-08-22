#!/bin/bash

docker run --rm -it \
    -u $(id -u) \
    --device /dev/kvm \
    --group-add=$(getent group kvm | cut -d : -f 3) \
    -v ${PWD}:/root \
    debos \
    /bin/bash -c "debos $*"
