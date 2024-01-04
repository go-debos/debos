# debos

Docker container for ['debos' tool](https://github.com/go-debos/debos).

## Installation
```
docker pull godebos/debos
```

Debos needs virtualization to be enabled on the host and shared with the container.

Check that `kvm` is enabled and writable by the user running the docker container by running ```ls /dev/kvm```

## Usage
/!\ This container should be used as an executable, i.e. there is no need to add `debos` after `godebos/debos`.

To build `recipe.yaml`:
```
cd <PATH_TO_RECIPE_DIR>
docker run --rm --interactive --tty --device /dev/kvm --user $(id -u) --workdir /recipes --mount "type=bind,source=$(pwd),destination=/recipes" --security-opt label=disable godebos/debos <RECIPE.yaml>
```

If debos fails to run the KVM fakemachine backend and the `/dev/kvm` device exists on your host, you may need to add the owning group of the device as a supplementary group of the container. This will work if `ls -l /dev/kvm` indicates that the owning group has read-write access to the device. Adding the supplementary group may be unsafe depending on the owning group of `/dev/kvm`, but it could be required depending on your login provider. To add the group, add `--group-add "$(stat -c '%g' /dev/kvm)"` to your `docker run` command before `godebos/debos`. See [Docker run reference -- Additional Groups](https://docs.docker.com/engine/reference/run/#additional-groups) for more information.

## Container build
To build the debos container image from current git branch:
```
docker build -f docker/Dockerfile -t godebos/debos .
```

## Tests

### Unit tests
Run unit tests:
```
docker-compose -f docker/unit-tests.test.yml up --build --exit-code-from=sut
```
