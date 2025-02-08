/*
InstallDpkg Action

Install packages from .deb files and their dependencies to the target rootfs
using 'apt'.

Dependencies will be satisfied first from the `packages` list (i.e. locally
available packages) and then from the target's configured apt repositories. If
`deps` is set to false, dependencies will not be installed and an error will be
thrown. TODO: check this

Attempting to downgrade packages which are already installed is not allowed and
will throw an error.

 # Yaml syntax:
 - action: install-dpkg
   origin: name
   recommends: bool
   unauthenticated: bool
   deps: bool
   packages:
     - package_path.deb
     - *.deb

Mandatory properties:

- packages -- list of package files to install from the filesystem (or named
origin). Resolves Unix-style glob patterns. If installing from a named origin,
e.g. the result of a download action, the package path will be automatically
generated from the origin contents and the `packages` property can be omitted.

Optional properties:

- origin -- reference to named file or directory. Defaults to recipe directory.

- recommends -- boolean indicating if suggested packages will be installed. Defaults to false.

- unauthenticated -- boolean indicating if unauthenticated packages can be installed. Defaults to false.

- update -- boolean indicating if `apt update` will be run. Default 'true'.

Example to install all packages from recipe subdirectory `pkgs/`:

 - action: install-dpkg
   description: Install Debian packages from local recipe
   packages:
     - pkgs/*.deb

Example to install named packages from recipe subdirectory `pkgs/`:

 - action: install-dpkg
   description: Install Debian packages from local recipe
   packages:
     - pkgs/bmap-tools_*_all.deb
     - pkgs/fakemachine_*_amd64.deb

Example to download and install a package:

 - action: download
   description: Install Debian package from url
   url: http://ftp.us.debian.org/debian/pool/main/b/bmap-tools/bmap-tools_3.5-2_all.deb
   name: bmap-tools-pkg

 - action: install-dpkg
   description: Install Debian package from url
   origin: bmap-tools-pkg
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

type InstallDpkgAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Unauthenticated  bool
	Update           bool
	Origin           string
	Packages         []string
}

func NewInstallDpkgAction() *InstallDpkgAction {
	a := &InstallDpkgAction{Update: true}
	return a
}

func (apt *InstallDpkgAction) Run(context *debos.DebosContext) error {
	aptCommand := wrapper.NewAptCommand(*context, "install-dpkg")

	/* check if named origin exists or fallback to RecipeDir if no origin set */
	var origin string = context.RecipeDir
	if len(apt.Origin) > 0 {
		var found bool
		if origin, found = context.Origins[apt.Origin]; !found {
			return fmt.Errorf("origin %s not found", apt.Origin)
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
		if len(apt.Packages) == 0 {
			return fmt.Errorf("no packages defined")
		}

		for _, pkg := range apt.Packages {
			// resolve globs
			source := path.Join(origin, pkg)
			matches, err := filepath.Glob(source)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return fmt.Errorf("file(s) not found after globbing: %s", pkg)
			}

			packages = append(packages, matches...)
		}
	} else {
		packages = append(packages, origin)
	}

	/* bind mount each package into rootfs & update the list with the
	 * path relative to the chroot */
	for idx, pkg := range packages {
		// check for duplicates after globbing
		for j := idx + 1; j < len(packages); j++ {
			if packages[j] == pkg {
				return fmt.Errorf("duplicate package found: %s", pkg)
			}
		}

		log.Printf("Installing %s", pkg)

		/* Only bind mount the package if the file is outside the rootfs */
		if strings.HasPrefix(pkg, context.Rootdir) {
			pkg = strings.TrimPrefix(pkg, context.Rootdir)
		} else {
			aptCommand.AddBindMount(pkg, "")
		}

		/* update pkg list with the complete resolved path */
		packages[idx] = pkg
	}

	if apt.Update {
		if err := aptCommand.Update(); err != nil {
			return err
		}
	}

	if err := aptCommand.Install(packages, apt.Recommends, apt.Unauthenticated); err != nil {
		return err
	}

	if err := aptCommand.Clean(); err != nil {
		return err
	}

	return nil
}
