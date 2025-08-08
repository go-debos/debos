/*
Apt Action

Install packages and their dependencies to the target rootfs with 'apt'.

 # Yaml syntax:
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

- recommends -- boolean indicating if suggested packages will be installed. Default 'false'.

- unauthenticated -- boolean indicating if unauthenticated packages can be installed. Default 'false'.

- update -- boolean indicating if `apt update` will be run. Default 'true'.
*/
package actions

import (
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/wrapper"
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
	aptCommand := wrapper.NewAptCommand(*context, "apt")

	if apt.Update {
		if err := aptCommand.Update(); err != nil {
			return err
		}
	}

	if err := aptCommand.Install(apt.Packages, apt.Recommends, apt.Unauthenticated); err != nil {
		return err
	}

	if err := aptCommand.Clean(); err != nil {
		return err
	}

	return nil
}
