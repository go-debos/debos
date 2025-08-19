/* Abstracts the apt command. */
package wrapper

import (
	"github.com/go-debos/debos"
)

type AptCommand struct {
	Wrapper
}

func NewAptCommand(context debos.DebosContext, label string) AptCommand {
	command := "apt-get"

	apt := AptCommand{
		Wrapper: NewCommandWrapper(context, command, label),
	}

	apt.AddEnv("DEBIAN_FRONTEND=noninteractive")

	/* Don't show progress update percentages */
	apt.AppendGlobalArguments("-o=quiet::NoUpdate=1")

	return apt
}

func (apt AptCommand) Clean() error {
	return apt.Run("clean")
}

func (apt AptCommand) Install(packages []string, recommends bool, unauthenticated bool) error {
	arguments := []string{"install", "--yes"}

	if !recommends {
		arguments = append(arguments, "--no-install-recommends")
	}

	if unauthenticated {
		arguments = append(arguments, "--allow-unauthenticated")
	}

	arguments = append(arguments, packages...)

	return apt.Run(arguments...)
}

func (apt AptCommand) Update() error {
	return apt.Run("update")
}
