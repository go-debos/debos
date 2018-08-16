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
	"fmt"
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

	if context.HttpProxy != "" {
		c.AddEnv(fmt.Sprintf("http_proxy=%s", context.HttpProxy))
	}

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
