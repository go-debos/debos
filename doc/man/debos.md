% debos(1)

# NAME

debos -  Debian OS images builder


# SYNOPSIS

```
debos [options] <recipe file in YAML>
debos [--help]
```

Application Options:

```
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
```

# DESCRIPTION

debos is a tool to make the creation of various Debian-based OS images
simpler. While most other tools focus on specific use-cases, debos is
designed to be a toolchain making common actions trivial while providing
enough rope to do whatever tweaking which might be required behind the
scenes.

debos expects a YAML file as input. A general overview of a YAML recipe and of
the templating engine used can be found in the
[debos recipe syntax documentation](https://pkg.go.dev/github.com/go-debos/debos/actions#hdr-Recipe_syntax).

debos runs the actions listed in the recipe file sequentially. These actions
should be self-contained and independent of each other.

Some of the actions provided by debos to customise and produce images are:

* `apt`: install packages and their dependencies with `apt`
* `debootstrap`: construct the target rootfs with `debootstrap`
* `download`: download a single file from the internet
* `filesystem-deploy`: deploy a root filesystem to an image previously created
* `image-partition`: create an image file, make partitions and format them
* `install-deb`: install packages and their dependencies from local deb packages
* `ostree-commit`: create an OSTree commit from rootfs
* `ostree-deploy`: deploy an OSTree branch to the image
* `overlay`: do a recursive copy of directories or files to the target filesystem
* `pack`: create a tarball with the target filesystem
* `pacman`: install packages and their dependencies with pacman
* `pacstrap`: construct the target rootfs with pacstrap
* `raw`: directly write a file to the output image at a given offset
* `recipe`: includes the recipe actions at the given path
* `run`: allows to run a command or script in the filesystem or in the host
* `unpack`: unpack files from archive in the filesystem

A full syntax description of all the debos actions can be found in the
[debos actions documentation](https://godoc.org/github.com/go-debos/debos/actions).

# GET IN TOUCH!

💬 Join us on Matrix at [#debos:matrix.debian.social](https://matrix.to/#/#debos:matrix.debian.social)
to chat about usage or development of debos.

🪲 To report a bug, issue or feature request, create a new
[GitHub Issue](https://github.com/go-debos/debos/issues).

❓ Please use the [GitHub Discussion forum](https://github.com/go-debos/debos/discussions)
to ask questions about how to use Debos or to discuss best ways of creating
recipes.

# INSTALLATION (DOCKER CONTAINER)

An official debos container is available:
```
docker pull godebos/debos
```

See [docker/README.md](docker/README.md) for usage.

# USING DEBOS IN GITHUB ACTIONS

debos can be run in GitHub Actions using the official container with KVM support
for isolated and reproducible builds. The `--fakemachine-backend=kvm` option is
specified to ensure KVM is used as expected:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/go-debos/debos:main
      options: --device=/dev/kvm
    steps: 
      - uses: actions/checkout@v4
      - run: debos --fakemachine-backend=kvm --print-recipe recipe.yaml
```

# INSTALLATION FROM SOURCE

## DEPENDENCIES

Debian:

```bash
sudo apt install golang git libglib2.0-dev libostree-dev qemu-system-x86 \
     qemu-user-static debootstrap systemd-container
```

Arch Linux:

```bash
# pipewire-jack is used to satisfy the jack dependency required by qemu-full,
# alternatively jack2 can be used instead.
sudo pacman -S --needed base-devel pkgconf go git glib2 ostree \
     gobject-introspection debootstrap qemu-full qemu-user-static \
     systemd pipewire-jack
```

## INSTALLING

```bash
export GOPATH=/opt/src/gocode # or whatever suits your needs

go install -v github.com/go-debos/debos/cmd/debos@latest

/opt/src/gocode/bin/debos --help
```

# SIMPLE EXAMPLE

The following example will create an arm64 image, install several
packages in it, change the file `/etc/hostname` to `debian` and finally
make a tarball of the complete system.

```yaml
{{- $image := or .image "debian.tgz" -}}

architecture: arm64

actions:
  - action: debootstrap
    suite: trixie
    components:
      - main
      - non-free-firmware
    mirror: https://deb.debian.org/debian
    variant: minbase

  - action: apt
    packages:
      - sudo
      - openssh-server
      - adduser
      - systemd-sysv
      - firmware-linux

  - action: run
    chroot: true
    command: echo debian > /etc/hostname

  - action: pack
    file: {{ $image }}
    compression: gz
```

To run it, create a file named `example.yaml` and run:

```bash
debos example.yaml
```

The final tarball will be named `debian.tgz`. If you would like to modify
the fileame, you can provide a different name for the variable image
like this:

```bash
debos -t image:"debian-arm64.tgz" example.yaml
```

# OTHER EXAMPLE RECIPES

See the [bundled example recipes](doc/examples) for some more detailed example
recipes. Additional more detailed example recipes are stored under [debos-recipes](https://github.com/go-debos/debos-recipes).

# ENVIRONMENT VARIABLES

debos reads a predefined list of environment variables from the host and
propagates them to the fakemachine build environment. The set of
environment variables is defined by `environ_vars` in
`cmd/debos/debos.go`. Currently the list of environment variables includes
the proxy environment variables documented at:

https://wiki.archlinux.org/index.php/proxy_settings

The list of environment variables currently exported to fakemachine is:

```
http_proxy, https_proxy, ftp_proxy, rsync_proxy, all_proxy, no_proxy
```

While the elements of `environ_vars` are in lower case, for each element
both lower and upper case variants are probed on the host and if found
propagated to fakemachine. So if the host has the environment variables
HTTP_PROXY and no_proxy defined, both will be propagated to fakemachine
respecting the case.

The command line options `--environ-var` and `-e` can be used to specify,
overwrite and unset environment variables for fakemachine with the syntax:

```bash
debos -e ENVIRONVAR:VALUE ...
```

To unset an environment variable, or in other words, to prevent an
environment variable being propagated to fakemachine, use the same syntax
without a value. debos accepts multiple -e simultaneously.

# PROXY CONFIGURATION

While the proxy related environment variables are exported from the host
to fakemachine, there are two known sources of issues:

* Using localhost will not work from fakemachine. Use an address which
  is valid on your network. debos will warn if the environment variables
  contain localhost.

* In case you are running applications and/or scripts inside fakemachine
  you may need to check which are the proxy environment variables they
  use. Different apps are known to use different environment variable
  names and different case for environment variable names.

# FAKEMACHINE BACKEND

debos (unless running debos with the `--disable-fakemachine` argument)
creates and spawns a virtual machine using [fakemachine](https://github.com/go-debos/fakemachine)
and executes the actions defined by the recipe inside the virtual machine.
This helps ensure recipes are reproducible no matter the host environment.

Fakemachine can use different virtualisation backends to spawn the virtual
machine, for more information see the [fakemachine documentation](https://github.com/go-debos/fakemachine).

By default the backend will automatically be selected based on what is
supported by the host machine, but this can be overridden using the
`--fakemachine-backend` / `-b` option. If no backends are supported,
debos reverts to running the recipe on the host without creating a
fakemachine.

Performance of the backends is roughly as follows: `kvm` is faster than
`uml` is faster than `qemu`. Using `--disable-fakemachine` is slightly
faster than `kvm`, but requires root permissions.

Benchmark times for running [pine-a64-plus/debian.yaml](https://github.com/go-debos/debos-recipes/blob/9a25b4be6c9136f4a27e542f39ab7e419fc852c9/pine-a64-plus/debian.yaml)
on an Intel Pentium G4560T with SSD:

| Backend | Wall Time | Prerequisites |
| --- | --- | --- |
| `--disable-fakemachine` | 8 min | root permissions |
| `-b kvm` | 9 min | access to `/dev/kvm` |
| `-b uml` | 18 min | package `user-mode-linux` installed  |
| `-b qemu` | 166 min | none |
