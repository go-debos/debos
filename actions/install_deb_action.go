/*
InstallDeb Action

Install packages from .deb files and their dependencies to the target rootfs
using 'apt'.

Dependencies will be satisfied first from the `packages` list (i.e. locally
available packages) and then from the target's configured apt repositories.

Attempting to downgrade packages which are already installed is not allowed and
will throw an error.

 # Yaml syntax:
 - action: install-deb
   origin: name
   recommends: bool
   unauthenticated: bool
   update: bool
   packages:
     - package_path.deb
     - *.deb

Optional properties:

- packages -- list of package files to install from the filesystem (or named
origin). Resolves Unix-style glob patterns. If installing from a named origin
with a single package (e.g. the result of a download action) the package list
will be automatically generated from the origin contents and the `packages`
property can be omitted.

- origin -- reference to named origin. Defaults to `recipe` to install from local filesystem.

- recommends -- boolean indicating if recommended packages will be installed. Defaults to false.

- unauthenticated -- boolean indicating if unauthenticated packages (e.g. dependencies) can be installed by apt. Defaults to false.

- update -- boolean indicating if `apt update` will be ran before installing packages. Defaults to true.

Example to install all packages from recipe subdirectory `pkgs/`:

 - action: install-deb
   description: Install Debian packages from local recipe
   packages:
     - pkgs/*.deb

Example to install named packages from recipe subdirectory `pkgs/`:

 - action: install-deb
   description: Install Debian packages from local recipe
   packages:
     - pkgs/bmap-tools_*_all.deb
     - pkgs/fakemachine_*_amd64.deb

Example to download and install a package:

 - action: download
   description: Install Debian package from url
   url: http://ftp.us.debian.org/debian/pool/main/b/bmap-tools/bmap-tools_3.5-2_all.deb
   name: bmap-tools-pkg

 - action: install-deb
   description: Install Debian package from url
   origin: bmap-tools-pkg
   packages:
     - bmap-tools_*_all.deb
*/

package actions

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/wrapper"
)

type InstallDebAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Unauthenticated  bool
	Update           bool
	Origin           string
	Packages         []string
}

func NewInstallDebAction() *InstallDebAction {
	return &InstallDebAction{Update: true}
}

func (act *InstallDebAction) Run(context *debos.Context) error {
	apt := wrapper.NewAptCommand(*context, "install-deb")

	/* check if named origin exists or fallback to RecipeDir if no origin set */
	var origin = context.RecipeDir
	if len(act.Origin) > 0 {
		var found bool
		if origin, found = context.Origins[act.Origin]; !found {
			return fmt.Errorf("origin %s not found", act.Origin)
		}
	}

	/* create a list of full paths of packages to install: if the origin is a
	 * single file (e.g download action) then just return that package, otherwise
	 * append package name to the origin path and glob to create a list of packages.
	 * In other words, install all packages which are in the origin's directory.
	 */
	packages := []string{}
	file, err := os.Stat(origin)
	if err != nil {
		return err
	}

	if file.IsDir() {
		if len(act.Packages) == 0 {
			return fmt.Errorf("no packages defined")
		}
		for _, pattern := range act.Packages {
			// resolve globs
			source := path.Join(origin, pattern)
			matches, err := filepath.Glob(source)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return fmt.Errorf("file(s) not found after globbing: %s", pattern)
			}

			packages = append(packages, matches...)
		}
	} else {
		if len(act.Packages) > 0 {
			return fmt.Errorf("packages cannot be used when origin points to a single file")
		}
		packages = append(packages, origin)
	}

	// de-duplicate the package list while preserving order
	seen := make(map[string]struct{}, len(packages))
	dedup := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		dedup = append(dedup, pkg)
	}
	packages = dedup

	/* bind mount each package into rootfs & update the list with the
	 * path relative to the chroot */
	for idx, pkg := range packages {
		log.Printf("Adding %s to install list", pkg)

		/* Only bind mount the package if the file is outside the rootfs */
		if strings.HasPrefix(pkg, context.Rootdir) {
			pkg = strings.TrimPrefix(pkg, context.Rootdir)
		} else {
			apt.AddBindMount(pkg, "")
		}

		/* update pkg list with the complete resolved path */
		packages[idx] = pkg
	}

	/* run apt update */
	if act.Update {
		if err := apt.Update(); err != nil {
			return err
		}
	}

	if err := apt.Install(packages, act.Recommends, act.Unauthenticated); err != nil {
		return err
	}

	if err := apt.Clean(); err != nil {
		return err
	}

	return nil
}
