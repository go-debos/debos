# Contributing to debos

:+1::tada: Firstly, thank you for taking the time to contribute! :tada::+1:

The following is a set of guidelines for contributing to debos, which is hosted on [GitHub](https://github.com/go-debos/debos).

These are mostly guidelines and not rules. Use your best judgement and feel free to propose changes to this document in a pull request.


## Get in touch!

💬 Join us on Matrix at [#debos:matrix.debian.social](https://matrix.to/#/#debos:matrix.debian.social)
to chat about usage or development of debos.

🪲 To report a bug or feature request, please create a new
[GitHub Issue](https://github.com/go-debos/debos/issues).
Issues here should be about debos itself (image generation, recipe actions, build tooling) - not about the OS images you're building or runtime behaviour of the resulting system.


## Maintainers

 - [Sjoerd Simons - @sjoerdsimons](https://github.com/sjoerdsimons)
 - [Christopher Obbard - @obbardc](https://github.com/obbardc)
 - [Dylan Aïssi - @daissi](https://github.com/daissi)


## Code of conduct

Be kind, constructive and respectful.


## Ways to contribute

- **Report bugs** and regressions
- **Improve documentation** (README, man pages, comments)
- **Add tests** or extend existing ones
- **Implement small features** or refactorings that improve maintainability

If you're planning a larger change, please open an issue first so it can be discussed with maintainers before investing a lot of time.


## Reporting bugs

Please create a [GitHub Issue](https://github.com/go-debos/debos/issues) and include:

- debos version (`debos --version`, or git commit/tag)
- Host distribution and version (e.g. Debian 12, Ubuntu 24.04)
- Architecture (e.g. `amd64`, `arm64`)
- Fakemachine backend you're using (e.g. `kvm`, `qemu`, `--disable-fakemachine`)
- The recipe file (or a minimal reproducer)
- Steps to reproduce
- What you expected to happen
- What actually happened (including **full** error output)


## Development setup

Prerequisites:

- A recent Go toolchain (matching the `go` version in `go.mod`)
- A POSIX shell and basic build tools
- Docker (for running the linter and recipe tests)
- Optional but recommended: access to `/dev/kvm` on your host

Clone your fork:

```sh
git clone https://github.com/<your-username>/debos.git
cd debos
```

Run the unit tests:

```sh
go test ./...
```

### Running the linter

The linter requires `libostree-dev`, which is most easily provided via Docker.
Run the following commands inside a golangci-lint container:

```sh
docker run --rm -it -v $(pwd):/app -w /app golangci/golangci-lint:v2.3.1 bash
apt update && apt install --yes libostree-dev
go build github.com/sjoerdsimons/ostree-go/pkg/otbuiltin
golangci-lint run
```

### Man page

If you modify any documentation that affects the man page, regenerate it:

```sh
cd doc/man/ && ./create_manpage.sh
```

Commit the updated man page alongside your changes. CI will fail if the man page is out of date.


## Coding style

debos is written in Go. Please follow the usual Go conventions:

* Format code with `gofmt` (or `go fmt ./...`)
* Keep changes **small and focused** where possible
* Prefer clear, simple code over clever one-liners
* Add or update tests when fixing bugs or adding behaviour

If you touch existing code, try to follow the style of the surrounding code.


## Submitting changes (Pull Requests)

1. Fork the repository and create a feature branch:

```sh
git checkout -b wip/my-username/my-feature
```

2. Make your changes and keep the commit history reasonably clean
   (small, logical commits are easier to review).

3. Ensure unit tests pass locally:

```sh
go test ./...
```

4. Ensure lint tests pass locally (see [Running the linter](#running-the-linter) above).

5. Push your branch and open a **Pull Request** against `main`:

   * Use a clear PR title (e.g. `actions/apt: fix recommends handling`)
   * Describe *what* you changed and *why*
   * Mention any relevant issue numbers (e.g. `Fixes: #123`)
   * Call out any behaviour changes or backward-incompatible changes

Reviewers may ask for adjustments - this is a normal part of the process.

Ensure your pull request is always rebased on top of the latest debos `main` branch.


## License and copyright

debos is licensed under the **Apache-2.0** License.
By submitting a pull request, you agree that your contributions will be licensed under the same terms.


Thanks again for helping improve debos!
