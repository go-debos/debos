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

## Container build
To build the debos container image from current git branch:
```
docker build -f docker/Dockerfile -t godebos/debos .
```

## Tests

### unit tests
Run unit test with debos-docker:
```
cd docker
docker-compose -f unit-tests.test.yml up --build
```
