#!/bin/sh

set -e

echo "I: create user"
adduser --gecos User user

echo "I: set user password"
echo "user:user" | chpasswd
adduser user sudo
