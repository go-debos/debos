package actions

import (
	"github.com/go-debos/debos"
)

type AptAction struct {
	debos.BaseAction `yaml:",inline"`
	Recommends bool
	Packages   []string
}

func (apt *AptAction) Run(context *debos.DebosContext) error {
	apt.LogStart()
	aptOptions := []string{"apt-get", "-y"}

	if !apt.Recommends {
		aptOptions = append(aptOptions, "--no-install-recommends")
	}

	aptOptions = append(aptOptions, "install")
	aptOptions = append(aptOptions, apt.Packages...)

	c := debos.NewChrootCommand(context.Rootdir, context.Architecture)
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
