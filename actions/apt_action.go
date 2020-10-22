/*
Apt Action

Install packages and their dependencies to the target rootfs with 'apt'.

Yaml syntax:
 - action: apt
   recommends: bool
   unauthenticated: bool
   local-packages-dir: directory
   packages:
     - package1
     - package2

Mandatory properties:

- packages -- list of packages to install

Optional properties:

- recommends -- boolean indicating if suggested packages will be installed

- unauthenticated -- boolean indicating if unauthenticated packages can be installed
- local-packages-dir -- directory containing local packages to be installed,
located in the recipe directory ($RECIPEDIR)
*/
package actions

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

type AptAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Unauthenticated  bool
	Packages         []string
	LocalPackagesDir string `yaml:"local-packages-dir"` // external path containing local packages
}

func (apt *AptAction) Run(context *debos.DebosContext) error {
	apt.LogStart()
	aptOptions := []string{"apt-get", "-y"}

	if !apt.Recommends {
		aptOptions = append(aptOptions, "--no-install-recommends")
	}

	if apt.Unauthenticated {
		aptOptions = append(aptOptions, "--allow-unauthenticated")
	}

	aptOptions = append(aptOptions, "install")
	aptOptions = append(aptOptions, apt.Packages...)

	if apt.LocalPackagesDir != "" {
		localpackagesdir := path.Join(context.RecipeDir, apt.LocalPackagesDir)
		destination, err := ioutil.TempDir(path.Join(context.Rootdir, "usr/local"), "debs")
		if err != nil {
			return err
		}
		defer os.RemoveAll(destination)

		err = debos.CopyTree(localpackagesdir, destination)
		if err != nil {
			return err
		}

		currentDir, _ := os.Getwd()
		os.Chdir(destination)

		// apt-ftparchive tries to read /etc/apt/apt.conf.d, add this path temporarily to prevent warnings
		os.MkdirAll("/etc/apt/apt.conf.d", 0755)
		defer os.RemoveAll("/etc/apt/apt.conf.d/")

		err = debos.Command{}.Run("apt", "sh", "-c", "apt-ftparchive packages . > Packages")
		if err != nil {
			return err
		}
		err = debos.Command{}.Run("apt", "sh", "-c", "apt-ftparchive release . > Release")
		if err != nil {
			return err
		}

		os.Chdir(currentDir)

		locallist, err := os.OpenFile(path.Join(context.Rootdir, "/etc/apt/sources.list.d/debos-local-debs.list"),
			os.O_RDWR | os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer os.RemoveAll(locallist.Name())
		_, err = locallist.WriteString(fmt.Sprintf("deb [trusted=yes] file://%s ./\n",
			strings.TrimPrefix(destination, context.Rootdir)))
		if err != nil {
			return err
		}
		locallist.Close()
	}

	c := debos.NewChrootCommandForContext(*context)
	c.AddEnv("DEBIAN_FRONTEND=noninteractive")

	err := c.Run("apt", "apt-get", "update")
	if err != nil {
		return err
	}
	err = c.Run("apt", aptOptions...)
	if err != nil {
		return err
	}
	err = c.Run("apt", "apt-get", "clean")
	if err != nil {
		return err
	}

	return nil
}
