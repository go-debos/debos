#!/bin/bash

TEST=0
FAILURES=0
VERBOSE=1

function test_failed {
    local MSG="$1"
    TEST=$(($TEST + 1))
    FAILURES=$(($FAILURES + 1))
    echo "Test ${TEST}: ${MSG}"
}

function test_passed {
    TEST=$(($TEST + 1))
    echo "Test ${TEST}: PASS"
}

function run_cmd {
    local CMD="$@"
    echo
    echo "Running ${CMD}"
    if [[ $VERBOSE == 0 ]]; then
        $CMD &>/dev/null
    else
        $CMD
    fi
    return $?
}

function expect_success {
    local CMD="$@"
    run_cmd $CMD && test_passed || test_failed "${CMD} failed with exitcode $?, expected success"
}

function expect_failure {
    local CMD="$@"
    run_cmd $CMD && test_failed "${CMD} succeeded, failure expected." || test_passed
}

function rename_command {
    newname="$1"
    shift
    (exec -a "$newname" "$@")
    return $?
}

if [ -v sudo ]; then
    SUDO=sudo
else
    SUDO=
fi

expect_success debos --help
expect_failure debos --not-a-valid-option
expect_failure debos
expect_failure debos good.yaml good.yaml
expect_failure debos --disable-fakemachine --fakemachine-backend=qemu good.yaml
expect_failure debos missing-file.yaml
expect_failure debos garbled.yaml
expect_failure debos --fakemachine-backend=kvm good.yaml
expect_failure debos verify-fail.yaml
expect_success debos --dry-run good.yaml
expect_failure debos --memory=NotANumber good.yaml
expect_failure debos --scratchsize=NotANumber good.yaml
expect_success debos good.yaml
expect_failure debos bad.yaml
expect_failure debos pre-machine-failure.yaml
expect_failure debos post-machine-failure.yaml
expect_failure debos overlay-missing-destination.yaml
expect_failure debos overlay-missing-source.yaml
expect_failure debos overlay-no-source.yaml
expect_failure debos missing-mountpoint.yaml
expect_failure debos missing-partition.yaml
expect_failure rename_command NOT_DEBOS debos good.yaml

expect_failure $SUDO debos missing-file.yaml --disable-fakemachine
expect_failure $SUDO debos garbled.yaml --disable-fakemachine
expect_failure $SUDO debos verify-fail.yaml --disable-fakemachine
expect_success $SUDO debos --dry-run good.yaml --disable-fakemachine
expect_success $SUDO debos good.yaml --disable-fakemachine
expect_failure $SUDO debos bad.yaml --disable-fakemachine
expect_failure $SUDO debos pre-machine-failure.yaml --disable-fakemachine
expect_failure $SUDO debos post-machine-failure.yaml --disable-fakemachine
expect_failure $SUDO debos overlay-missing-destination.yaml --disable-fakemachine
expect_failure $SUDO debos overlay-missing-source.yaml --disable-fakemachine
expect_failure $SUDO debos unknown-property.yaml --disable-fakemachine
expect_failure $SUDO debos unknown-property-recipe.yaml --disable-fakemachine

echo
if [[ $FAILURES -ne 0 ]]; then
    SUCCESSES=$(( $TEST - $FAILURES ))
    echo "Error: Only $SUCCESSES/$TEST tests passed"
    exit 1
fi

echo "All tests passed"
