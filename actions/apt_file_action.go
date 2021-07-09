/*
AptFile Action

Install packages from .deb files and their dependencies to the target rootfs
with 'apt'.

Dependencies will be satisfied first from the package list and then from the
target's configured apt repositories.

Attempting to downgrade packages which are already installed is not allowed and
will throw an error.

Yaml syntax:
 - action: apt-file
   origin: name
   recommends: bool
   unauthenticated: bool
   packages:
     - package1
     - package2

Mandatory properties:

- packages -- list of packages to install. Resolves Unix-style glob patterns.

Optional properties:

- origin -- reference to named file or directory. Defaults to  recipe directory.

- recommends -- boolean indicating if suggested packages will be installed. Defaults to false.

- unauthenticated -- boolean indicating if unauthenticated packages can be  installed. Defaults to false.


Example to install named packages in a subdirectory under `debs/`:

 - action: apt-file
   description: Test install from file
   packages:
     - pkgs/bmap-tools_*_all.deb
     - pkgs/fakemachine_*_amd64.deb


Example to download and install a package:

 - action: download
   url: http://ftp.us.debian.org/debian/pool/main/b/bmap-tools/bmap-tools_3.5-2_all.deb
   name: bmap-tools-pkg

 - action: apt-file
   description: Test install from download
   origin: bmap-tools-pkg

*/
package actions

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"github.com/go-debos/debos"
)

type AptFileAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Unauthenticated  bool
	Origin           string
	Packages         []string
}

func (apt *AptFileAction) Run(context *debos.DebosContext) error {
	apt.LogStart()
	var origin string
	aptOptions := []string{"apt", "-oDpkg::Progress-Fancy=0", "--yes"}
	pkgs := []string{}

	c := debos.NewChrootCommandForContext(*context)
	c.AddEnv("DEBIAN_FRONTEND=noninteractive")

	// get the full path of a named origin
	if len(apt.Origin) > 0 {
		var found bool
		if origin, found = context.Origins[apt.Origin]; !found {
			return fmt.Errorf("Origin not found '%s'", apt.Origin)
		}
	} else {
		// otherwise fallback to RecipeDir
		origin = context.RecipeDir
	}

	/* create a list of full paths of packages to install: if the origin is a
	 * single file (e.g download action) then just return that package, otherwise
	 * append package name to the origin path and glob to create a list of packages */
	file, err := os.Stat(origin)
	if err != nil {
		return err
	}
	if file.IsDir() {
		if len(apt.Packages) == 0 {
			return fmt.Errorf("No packages defined")
		}

		for _, pkg := range apt.Packages {
			// resolve globs
			source := path.Join(origin, pkg)
			matches, err := filepath.Glob(source)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return fmt.Errorf("File(s) not found after globbing: %s", pkg)
			}

			pkgs = append(pkgs, matches...)
		}
	} else {
		pkgs = append(pkgs, origin)
	}

	/* bind mount each package into rootfs & update the list with the
	 * path relative to the chroot */
	for idx, pkg := range pkgs {
		// check for duplicates after globbing
		for j := idx + 1; j < len(pkgs); j++ {
			if pkgs[j] == pkg {
				return fmt.Errorf("Duplicate package found: %s", pkg)
			}
		}

		// only bind-mount if the package is outside the rootfs
		if strings.HasPrefix(pkg, context.Rootdir) {
			pkg = strings.TrimPrefix(pkg, context.Rootdir)
		} else {
			c.AddBindMount(pkg, "")
		}

		// update pkgs with the resolved path
		pkgs[idx] = "." + pkg
	}

	err = c.Run("apt-file", "apt-get", "update")
	if err != nil {
		return err
	}

	if !apt.Recommends {
		aptOptions = append(aptOptions, "--no-install-recommends")
	}

	if apt.Unauthenticated {
		aptOptions = append(aptOptions, "--allow-unauthenticated")
	}

	aptOptions = append(aptOptions, "install")
	aptOptions = append(aptOptions, pkgs...)
	err = c.Run("apt-file", aptOptions...)
	if err != nil {
		return err
	}

	err = c.Run("apt-file", "apt-get", "clean")
	if err != nil {
		return err
	}

	return nil
}
