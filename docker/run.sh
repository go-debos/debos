#!/bin/bash

docker run --rm \
	-it \
	--privileged \
	-v ${PWD}:/root \
	debos \
	/bin/bash -c "debos $*"
