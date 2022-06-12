/*
Apt Action

Install packages and their dependencies to the target rootfs with 'apt'.

Yaml syntax:
 - action: apt
   recommends: bool
   unauthenticated: bool
   update: bool
   packages:
     - package1
     - package2

Mandatory properties:

- packages -- list of packages to install

Optional properties:

- recommends -- boolean indicating if suggested packages will be installed

- unauthenticated -- boolean indicating if unauthenticated packages can be installed

- update -- boolean indicating if `apt update` will be run. Default 'true'.
*/
package actions

import (
	"github.com/go-debos/debos"
)

type AptAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends       bool
	Unauthenticated  bool
	Update           bool
	Packages         []string
}

func NewAptAction() *AptAction {
	a := &AptAction{Update: true}
	return a
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

	c := debos.NewChrootCommandForContext(*context)
	c.AddEnv("DEBIAN_FRONTEND=noninteractive")

	if apt.Update {
		err := c.Run("apt", "apt-get", "update")
		if err != nil {
			return err
		}
	}

	err := c.Run("apt", aptOptions...)
	if err != nil {
		return err
	}
	err = c.Run("apt", "apt-get", "clean")
	if err != nil {
		return err
	}

	return nil
}
