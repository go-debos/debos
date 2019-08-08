# debos -  Debian OS images builder

## Sypnosis

    debos [options] <recipe file in YAML>
    debos [--help]

Application Options:

          --artifactdir=
      -t, --template-var=   Template variables
          --debug-shell     Fall into interactive shell on error
      -s, --shell=          Redefine interactive shell binary (default: bash)
          --scratchsize=    Size of disk backed scratch space
      -e, --environ-var=    Environment variables
      -v, --verbose         Verbose output
          --print-recipe    Print final recipe
          --dry-run         Compose final recipe to build but without any real work started


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
* pkglist: export a Debian package list to the artifacts directory
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
    export GOPATH=/opt/src/gocode # or whatever suites your needs
    go get -u github.com/go-debos/debos/cmd/debos
    /opt/src/gocode/bin/debos --help

## Simple example

The following example will create a arm64 image, install several
packages in it, change the file /etc/hostname to "debian" and finally
make a tarball.

    {{- $image := or .image "debian.tgz" -}}

    architecture: arm64

    actions:
      - action: debootstrap
        suite: "buster"
        components:
          - main
          - non-free
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

This example builds a customized image for a Raspberry Pi 3.
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

## See also
fakemachine at https://github.com/go-debos/fakemachine
