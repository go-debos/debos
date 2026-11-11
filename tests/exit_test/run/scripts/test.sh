#!/bin/sh

echo "test from script; arguments: $@"

# Ensure arguments are passed through to the script
if [ "$#" -ne 0 ]; then
    [ "$#" -eq 2 ] && [ "$1" = "argument1" ] && [ "$2" = "argument2" ]
fi
