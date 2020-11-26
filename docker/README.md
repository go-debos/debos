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
docker run --rm --interactive --tty \
  --device /dev/kvm \
  --user $(id -u):$(id -g) \
  --group-add $(getent group kvm | cut -d: -f3) \
  --workdir /recipes \
  --mount "type=bind,source=$(pwd),destination=/recipes" \
  --security-opt label=disable \
  godebos/debos recipe.yaml
```

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
