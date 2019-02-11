/*
Apt Action

Install packages and their dependencies to the target rootfs with 'apt'.

Yaml syntax:
 - action: apt
   recommends: bool
   packages:
     - package1
     - package2

Mandatory properties:

- packages -- list of packages to install

Optional properties:

- recommends -- boolean indicating if suggested packages will be installed
*/
package actions

import (
	"github.com/go-debos/debos"
)

type AptAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Packages         []string
}

func (apt *AptAction) Run(context *debos.DebosContext) error {
	apt.LogStart()
	aptOptions := []string{"apt-get", "-y"}

	if !apt.Recommends {
		aptOptions = append(aptOptions, "--no-install-recommends")
	}

	aptOptions = append(aptOptions, "install")
	aptOptions = append(aptOptions, apt.Packages...)

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
