# debos -  Debian OS images builder

## Synopsis

    debos [options] <recipe file in YAML>
    debos [--help]

Application Options:

      -b, --fakemachine-backend=   Fakemachine backend to use (default: auto)
          --artifactdir=           Directory for packed archives and ostree repositories (default: current directory)
      -t, --template-var=          Template variables (use -t VARIABLE:VALUE syntax)
          --debug-shell            Fall into interactive shell on error
      -s, --shell=                 Redefine interactive shell binary (default: bash) (default: /bin/bash)
          --scratchsize=           Size of disk backed scratch space
      -c, --cpus=                  Number of CPUs to use for build VM (default: 2)
      -m, --memory=                Amount of memory for build VM (default: 2048MB)
          --show-boot              Show boot/console messages from the fake machine
      -e, --environ-var=           Environment variables (use -e VARIABLE:VALUE syntax)
      -v, --verbose                Verbose output
          --print-recipe           Print final recipe
          --dry-run                Compose final recipe to build but without any real work started
          --disable-fakemachine    Do not use fakemachine.


## Description

debos is a tool to make the creation of various Debian-based OS images
simpler. While most other tools focus on specific use-cases, debos is
more meant as a tool-chain to make common actions trivial while providing
enough rope to do whatever tweaking that might be required behind the scene.

debos expects a YAML file as input and will run the actions listed in the
file sequentially. These actions should be self-contained and independent
of each other.

Some of the actions provided by debos to customize and produce images are:

* apt: install packages and their dependencies with 'apt'
* debootstrap: construct the target rootfs with debootstrap
* download: download a single file from the internet
* filesystem-deploy: deploy a root filesystem to an image previously created
* image-partition: create an image file, make partitions and format them
* ostree-commit: create an OSTree commit from rootfs
* ostree-deploy: deploy an OSTree branch to the image
* overlay: do a recursive copy of directories or files to the target filesystem
* pack: create a tarball with the target filesystem
* pacstrap: construct the target rootfs with pacstrap
* raw: directly write a file to the output image at a given offset
* recipe: includes the recipe actions at the given path
* run: allows to run a command or script in the filesystem or in the host
* unpack: unpack files from archive in the filesystem

A full syntax description of all the debos actions can be found at:
https://godoc.org/github.com/go-debos/debos/actions

## Installation (Docker container)

Official debos container is available:
```
docker pull godebos/debos
```

See [docker/README.md](https://github.com/go-debos/debos/blob/master/docker/README.md) for usage.

## Installation (under Debian)

    sudo apt install golang git libglib2.0-dev libostree-dev qemu-system-x86 \
         qemu-user-static debootstrap systemd-container
    export GOPATH=/opt/src/gocode # or whatever suits your needs
    go install -v github.com/go-debos/debos/cmd/debos@latest
    /opt/src/gocode/bin/debos --help

## Simple example

The following example will create a arm64 image, install several
packages in it, change the file /etc/hostname to "debian" and finally
make a tarball.

    {{- $image := or .image "debian.tgz" -}}

    architecture: arm64

    actions:
      - action: debootstrap
        suite: bookworm
        components:
          - main
        mirror: https://deb.debian.org/debian
        variant: minbase

      - action: apt
        packages: [ sudo, openssh-server, adduser, systemd-sysv, firmware-linux ]

      - action: run
        chroot: true
        command: echo debian > /etc/hostname

      - action: pack
        file: {{ $image }}
        compression: gz

To run it, create a file named `example.yaml` and run:

    debos example.yaml

The final tarball will be named "debian.tgz" if you would like to modify
this name, you can provided a different name for the variable image like
this:

    debos -t image:"debian-arm64.tgz" example.yaml

## Other examples

Example recipes are collected in a separate repository:

https://github.com/go-debos/debos-recipes

## Environment variables

debos read a predefined list of environment variables from the host and
propagates it to fakemachine. The set of environment variables is defined by
environ_vars on cmd/debos/debos.go. Currently the list of environment variables
includes the proxy environment variables as documented at:

https://wiki.archlinux.org/index.php/proxy_settings

The list of environment variables currently exported to fakemachine is:

    http_proxy, https_proxy, ftp_proxy, rsync_proxy, all_proxy, no_proxy

While the elements of environ_vars are in lower case, for each element both
lower and upper case variants are probed on the host, and if found propagated
to fakemachine. So if the host has the environment variables HTTP_PROXY and
no_proxy defined, both will be propagated to fakemachine respecting the case.

The command line options --environ-var and -e can be used to specify,
overwrite, and unset environment variables for fakemachine with the syntax:

    $ debos -e ENVIRONVAR:VALUE ...

To unset an enviroment variable, or in other words, to prevent an environment
variable to be propagated to fakemachine, use the same syntax without a value.
debos accept multiple -e simultaneously.

## Proxy configuration

While the proxy related environment variables are exported from the host to
fakemachine, there are two known sources of issues:

* Using localhost will not work from fakemachine. Prefer using an address that is valid on your network. debos will warn if environment variables contain localhost.

* In case you are running applications and/or scripts inside fakemachine you may need to check which are the proxy environment variables they use. Different apps are known to use different environment variable names and different case for environment variable names.

## Fakemachine Backend

debos (unless running debos with the `--disable-fakemachine` argument) creates
and spawns a virtual machine using [fakemachine](https://github.com/go-debos/fakemachine)
and executes the actions defined by the recipe inside the virtual machine. This
helps ensure recipes are reproducible no matter the host environment.

Fakemachine can use different virtualisation backends to spawn the virtualmachine,
for more information see the documentation under the [fakemachine repository](https://github.com/go-debos/fakemachine).

By default the backend will automatically be selected based on what is supported
on the host machine, but this can be overridden using the `--fakemachine-backend` / `-b`
option. If no backends are supported, debos reverts to running the recipe on the
host without creating a fakemachine.

Performance of the backends is roughly as follows: `kvm` is faster than `uml` is faster than `qemu`.
Using `--disable-fakemachine` is slightly faster than `kvm`, but requires root permissions.

Numbers for running [pine-a64-plus/debian.yaml](https://github.com/go-debos/debos-recipes/blob/9a25b4be6c9136f4a27e542f39ab7e419fc852c9/pine-a64-plus/debian.yaml) on an Intel Pentium G4560T with SSD:

| Backend | Wall Time | Prerequisites |
| --- | --- | --- |
| `--disable-fakemachine` | 8 min | root permissions |
| `-b kvm` | 9 min | access to `/dev/kvm` |
| `-b uml` | 18 min | package `user-mode-linux` installed  |
| `-b qemu` | 166 min | none |
