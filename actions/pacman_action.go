/*
Pacman Action

Install packages and their dependencies to the target rootfs with 'pacman'.

	# Yaml syntax:
	- action: pacman
	  packages:
	    - package1
	    - package2

Mandatory properties:

- packages -- list of packages to install
*/
package actions

import (
	"github.com/go-debos/debos"
)

type PacmanAction struct {
	debos.BaseAction `yaml:",inline"`
	Packages         []string
}

func (p *PacmanAction) Run(context *debos.DebosContext) error {
	p.LogStart()

	pacmanOptions := []string{"pacman", "-Syu", "--noconfirm"}
	pacmanOptions = append(pacmanOptions, p.Packages...)

	c := debos.NewChrootCommandForContext(*context)
	if err := c.Run("pacman", pacmanOptions...); err != nil {
		return err
	}

	return nil
}
