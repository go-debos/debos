# Integration Testing Guide

This document provides instructions for running debos integration tests using Docker.

## Quick Start

### 1. Build the Docker Image

```bash
# Standard build (for local development)
docker build --network=host -t debos -f docker/Dockerfile .

# For CI environments with MITM proxies (like GitHub Actions)
# Note: This path is specific to GitHub Actions runners
export MKCERT_CA=/home/runner/work/_temp/runtime-logs/mkcert/rootCA.pem
cp $MKCERT_CA docker/mkcert-ca.crt 2>/dev/null || true
docker build --network=host -t debos -f docker/Dockerfile .
```

### 2. Run Integration Tests

Basic test (quick validation):
```bash
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v recipes/test.yaml
```

All available tests:
```bash
# Recipe loading and overlay operations
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v recipes/test.yaml

# Templating and variable substitution
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v templating/test.yaml

# Partition operations and raw data writes
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v raw/test.yaml

# Partition table operations
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v partitioning/test.yaml

# MSDOS partition table
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v msdos/test.yaml
```

### 3. Verify Test Results

All tests should complete with:
```
==== Recipe done ====
```

And exit code 0.

## Test Categories

### Quick Tests (< 10 seconds)
- **recipes** - Recipe inclusion and overlay operations
- **templating** - Variable substitution and template functions

### Medium Tests (10-60 seconds)
- **raw** - Partition creation and raw data operations
- **partitioning** - Partition table operations
- **msdos** - MSDOS partition table handling

### Slow Tests (> 60 seconds, require network)
- **debian** - Debian debootstrap operations (requires network)
- **arch** - Arch Linux pacstrap operations (requires network)
- **apertis** - Apertis mmdebstrap operations (requires network)

## Troubleshooting

### Build Issues
See `docker/README-BUILD-ISSUES.md` for detailed troubleshooting of:
- Certificate errors (x509: certificate signed by unknown authority)
- Network failures (gitlab.archlinux.org unreachable)
- General Docker build issues

### Test Failures

**KVM not available:**
```
Error: KVM not available
```
Solution: Ensure `/dev/kvm` is accessible and you're in the `kvm` group

**Permission denied:**
```
Error: permission denied while trying to connect to /dev/kvm
```
Solution: Add `--privileged` flag to docker run

**Out of space:**
```
Error: no space left on device
```
Solution: Increase tmpfs size or use host directory instead of tmpfs

## CI/CD Integration

For GitHub Actions or similar CI environments:

```yaml
- name: Copy mkcert CA for Docker build
  run: |
    # GitHub Actions specific path for mkcert CA
    MKCERT_CA=/home/runner/work/_temp/runtime-logs/mkcert/rootCA.pem
    cp $MKCERT_CA docker/mkcert-ca.crt 2>/dev/null || true

- name: Build Docker image
  run: docker build --network=host -t debos -f docker/Dockerfile .

- name: Run quick validation tests
  run: |
    cd tests
    docker run --rm --device /dev/kvm \
      -v $(pwd):/tests -w /tests \
      --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
      debos -v recipes/test.yaml
```

## Development Workflow

When making changes to debos actions:

1. Make your code changes
2. Build the Docker image
3. Run relevant integration tests
4. Verify tests pass before submitting PR

Example for modifying the `overlay` action:
```bash
# 1. Make changes to actions/overlay_action.go

# 2. Build Docker image
docker build --network=host -t debos -f docker/Dockerfile .

# 3. Run tests that use overlay action
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v recipes/test.yaml

# 4. Verify all overlay operations work correctly
```

## Test Structure

Each test directory contains:
- `test.yaml` - Main recipe file
- Supporting files (tarballs, overlays, scripts, etc.)
- Expected to complete without errors

Test recipes demonstrate:
- Action functionality
- Edge cases
- Integration between actions
- Error handling (in exit_test)

## Advanced Usage

### Running with Different Backends

```bash
# Use QEMU backend (slower but more compatible)
docker run --rm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v --fakemachine-backend=qemu recipes/test.yaml

# Disable fakemachine (runs directly in container)
docker run --rm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v --disable-fakemachine recipes/test.yaml
```

### Running with Template Variables

```bash
# Pass template variables to test
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v -t sectorsize:4096 raw/test.yaml
```

### Debugging Tests

```bash
# Enable verbose output (use -v multiple times for increased verbosity)
# First -v enables verbose mode, second -v increases detail level
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v -v recipes/test.yaml

# Run with debug shell on failure
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v --shell=on-failure recipes/test.yaml
```

## Additional Resources

- Main documentation: `README.md`
- Docker build issues: `docker/README-BUILD-ISSUES.md`
- Copilot instructions: `.github/copilot-instructions.md`
- Action documentation: `actions/actions_doc.go`
