# Debos Repository - Copilot Coding Instructions

## Project Overview

**debos** is a tool for creating Debian-based OS images. It reads YAML recipe files and executes actions sequentially to build customized OS images. It uses fakemachine (a virtualization backend) to ensure reproducibility across different host environments.

**Repository Stats:**
- Language: Go (requires Go 1.23+, confirmed working with Go 1.24.7)
- Size: ~14MB (excluding build artifacts)
- Type: Command-line tool
- Architecture: 30 Go source files organized in actions-based architecture

## System Dependencies

**CRITICAL: Must install system dependencies before building:**

```bash
sudo apt-get update && sudo apt-get install -y \
    libglib2.0-dev \
    libostree-dev \
    pkg-config
```

Building without these dependencies will fail with pkg-config errors about missing glib-2.0 and gobject-2.0.

## Build Instructions

### Clean Build Sequence (ALWAYS follow this order)

1. **Download dependencies:**
   ```bash
   go mod download
   ```

2. **Tidy modules (if go.mod changed):**
   ```bash
   go mod tidy
   ```

3. **Pre-build ostree package (REQUIRED before main build):**
   ```bash
   go build github.com/sjoerdsimons/ostree-go/pkg/otbuiltin
   ```
   This step is necessary due to CGO dependencies in the ostree-go package. Skip this and the main build may fail intermittently.

4. **Build debos binary:**
   ```bash
   go build ./cmd/debos
   ```
   Or with version info:
   ```bash
   DEBOS_VER=$(git describe --always --tags HEAD)
   go build -ldflags="-X main.Version=${DEBOS_VER}" ./cmd/debos
   ```

5. **Verify build:**
   ```bash
   ./debos --version
   ```

**Build time:** ~30-60 seconds on modern hardware with cached dependencies.

### Running Tests

**Unit tests (fast, ~1-2 seconds):**
```bash
go test -v ./...
```

All tests should pass. CI requires that no tests are skipped (`! grep -q SKIP test.out`).

**Integration tests:** Integration/recipe tests run in Docker containers with fakemachine. Before submitting changes, run relevant integration test recipes, especially those containing actions that were modified. When adding new features, ensure they are exercised as part of an integration test. More information on how to run Docker based tests can be found later in this file.

**Testing focus:**
- When adjusting actions, focus on integration tests to verify the action behavior end-to-end
- Unit tests should only be added for specific subroutines containing complex computations

### Linting

**ALWAYS run linting before committing:**

1. **Install golangci-lint v2.3.1:**
   ```bash
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
       sh -s -- -b $(go env GOPATH)/bin v2.3.1
   ```

2. **Run linter:**
   ```bash
   golangci-lint run
   ```
   Or if installed in GOPATH:
   ```bash
   $(go env GOPATH)/bin/golangci-lint run
   ```

Configuration is in `.golangci.yml`. Enabled linters: govet, errorlint, misspell, revive, staticcheck, whitespace, gofmt.

**Expected result:** `0 issues.`

## Project Structure

### Root Directory Files
```
.gitignore           - Ignore patterns for Go, Linux, Vim, VS Code
.golangci.yml        - Linter configuration
README.md            - User documentation with installation and usage
TODO                 - Future feature ideas (informational only)
go.mod, go.sum       - Go module dependencies
action.go            - Core Action interface and Context definitions
commands.go          - Command execution and chroot handling
archiver.go          - Archive handling (tar, gz, etc.)
filesystem.go        - Filesystem operations
net.go               - Network operations (download support)
os.go                - OS-level utilities
debug.go             - Debug shell support
```

### Key Directories

**`cmd/debos/`** - Main entry point
- `debos.go` - CLI argument parsing, fakemachine setup, recipe execution

**`actions/`** - Action implementations (the core functionality)
- Each action type has its own file: `apt_action.go`, `debootstrap_action.go`, `download_action.go`, etc.
- `actions_doc.go` - Documentation for all action types
- Available actions: apt, debootstrap, mmdebstrap, download, filesystem-deploy, image-partition, ostree-commit, ostree-deploy, overlay, pack, pacman, pacstrap, raw, recipe, run, unpack

**`tests/`** - Integration test recipes
- Each subdirectory contains a `test.yaml` recipe file
- Tests: recipes, templating, partitioning, msdos, debian, arch, apertis, raw, exit_test
- These run in CI using Docker + fakemachine

**`doc/`** - Documentation and examples
- `doc/examples/` - Example recipe files (e.g., ospack-debian)
- `doc/man/` - Man page generation

**`docker/`** - Docker container build files
- `Dockerfile` - Multi-stage build for debos container
- `unit-tests.test.yml`, `exitcode-test.yml` - Docker Compose test configs

**`.github/workflows/`** - CI/CD pipeline
- `ci.yaml` - Comprehensive CI with lint, test, build, recipe-tests, example-recipes

### Architecture Overview

debos uses an **action-based architecture**:
1. Parse YAML recipe file
2. Create Context (scratchdir, rootdir, artifactdir, image, etc.)
3. Execute actions sequentially (each implements the Action interface)
4. Each action has lifecycle: Verify → PreMachine → Run → Cleanup → PostMachine
5. Actions run inside fakemachine VM for isolation (unless --disable-fakemachine)

The `Action` interface (in `action.go`) defines: Verify, PreMachine, PreNoMachine, Run, Cleanup, PostMachine, PostMachineCleanup.

**Note:** The action lifecycle differs when running with or without fakemachine (--disable-fakemachine). PreMachine is called when using fakemachine, while PreNoMachine is called when not using fakemachine.

## CI/CD Pipeline

The `.github/workflows/ci.yaml` runs:

1. **golangci** job - Linting in Debian trixie container
   - `go mod tidy`
   - `go build github.com/sjoerdsimons/ostree-go/pkg/otbuiltin` (pre-build required!)
   - `golangci-lint` with v2.3.1

2. **test** job - Matrix of 4 variants (arch, bookworm, trixie, forky)
   - Build with version: `go build -ldflags="-X main.Version=${DEBOS_VER}" ./cmd/debos`
   - Run unit tests: `go test -v ./...`
   - Verify no skipped tests: `! grep -q SKIP test.out`

3. **build** job - Docker container build for linux/amd64 and linux/arm64

4. **unit-tests** job - Runs unit tests in Docker builder stage

5. **recipe-tests** job - Extensive matrix of recipe tests with different backends (nofakemachine, qemu, uml, kvm)

6. **example-recipes** job - Tests example recipes (ospack-debian)

**All jobs must pass** before merging (enforced by `allgreen` job).

## Common Issues and Workarounds

### Build Failures

**Issue:** `Package glib-2.0 was not found in the pkg-config search path`
**Solution:** Install system dependencies (see System Dependencies section above)

**Issue:** Intermittent build failures with ostree-go
**Solution:** Always pre-build ostree package: `go build github.com/sjoerdsimons/ostree-go/pkg/otbuiltin`

### Code Patterns

- Uses `fakemachine` for VM isolation - do not modify fakemachine behavior without understanding impact
- CGO is used for glib/ostree bindings - changes to these areas need system library awareness
- YAML parsing - maintain YAML syntax compatibility in actions
- Template variables use `github.com/go-task/slim-sprig/v3` for templating

### Known TODOs/Hacks in Code

From codebase search:
- `net.go`: Proxy support TODO
- `action.go`: Verify method naming (FIXME)
- `actions/debootstrap_action.go`: Contains HACK comment
- `actions/ostree_deploy_action.go`: Multiple HACKs for repository handling and GPG signing
- `actions/raw_action.go`: TODO for deprecated syntax removal
- `actions/image_partition_action.go`: TODO about partition handling

These are existing issues - do not "fix" them unless specifically tasked to do so.

## Validation Checklist

Before submitting changes:

1. ✅ Install system dependencies (libglib2.0-dev, libostree-dev, pkg-config)
2. ✅ Run `go mod tidy` if dependencies changed
3. ✅ Pre-build ostree: `go build github.com/sjoerdsimons/ostree-go/pkg/otbuiltin`
4. ✅ Build succeeds: `go build ./cmd/debos`
5. ✅ Unit tests pass: `go test -v ./...`
6. ✅ Linter passes: `golangci-lint run` (0 issues)
7. ✅ Binary runs: `./debos --version`
8. ✅ **Integration tests pass**: Run relevant recipe tests for any actions modified (see Testing focus above)
9. ✅ This file should be updated if anything documented in it is changed

### Running Docker-Based Integration Tests

**CRITICAL:** When modifying action implementations, you MUST run Docker-based integration tests to validate changes before submitting:

1. **Build local Docker image with debos changes**:
   ```bash
   # Standard build (uses default Go module proxy behavior)
   docker build --network=host -t debos -f docker/Dockerfile .
   ```
   
   **Note for CI environments with MITM proxies:** If you encounter certificate validation errors during the build, you can bypass the Go module proxy by setting `GOPROXY=direct`:
   ```bash
   # Bypass Go proxy to avoid certificate issues
   docker build --network=host --build-arg GOPROXY=direct -t debos -f docker/Dockerfile .
   ```

2. **Run integration tests** with the local docker image:
   ```bash
   # Mount your locally-built debos binary into the container
   docker run --rm --device /dev/kvm \
     -v $(pwd)/tests:/tests -w /tests \
     --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
     debos -t <optional template variable> -v <test-name>/test.yaml
   ```

3. **Verify test results**: Tests should complete successfully, not just pass initial validation stages

**Common Issues:**
- **Docker build network errors**: Use `--network=host` flag to allow direct network access
- **Certificate validation errors**: Use `--build-arg GOPROXY=direct` to bypass proxy issues in CI environments
- **KVM access**: Ensure `/dev/kvm` is accessible and you're in the `kvm` group

Example test commands for action changes:
```bash
# Simple recipe test (quick validation):
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v recipes/test.yaml

# For mmdebstrap action changes using the apertis test:
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v -t tool:mmdebstrap apertis/test.yaml

# For debootstrap action changes by using the debian tests:
docker run --rm --cgroupns=private --device /dev/kvm --privileged \
  -v $(pwd)/tests:/tests -v $(pwd)/debos:/tmp/debos-test:ro \
  -w /tests --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v debian/test.yaml
```

For detailed troubleshooting of Docker build issues, see `docker/README-BUILD-ISSUES.md`.

## Git Workflow

- Main branch receives PRs
- The `.gitignore` excludes the `debos` binary, `*.test`, `*.out`, Go workspace files
- CI runs on all PRs and must pass

## Important Notes

- **Trust these instructions first** - only search/explore if information here is incomplete or incorrect
- **Pre-building ostree is not optional** - it prevents intermittent CGO build failures
- **System dependencies are required** - there is no pure-Go fallback for glib/ostree
- Changes to action implementations should maintain backward compatibility with existing recipes
- **When modifying actions, update their documentation** - Each action has inline documentation in its source file that must be kept in sync with code changes
- The project uses fakemachine for reproducibility - respect this design choice
