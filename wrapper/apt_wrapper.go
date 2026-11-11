package wrapper

import (
	"github.com/go-debos/debos"
)

// AptCommand wraps apt-get.
type AptCommand struct {
	Wrapper
}

// NewAptCommand returns an AptCommand for the context, logging with label.
func NewAptCommand(context debos.Context, label string) AptCommand {
	command := "apt-get"

	apt := AptCommand{
		Wrapper: NewCommandWrapper(context, command, label),
	}

	apt.AddEnv("DEBIAN_FRONTEND=noninteractive")

	/* Don't show progress update percentages; run dpkg without a pty so
	 * it doesn't emit carriage-return progress updates into the log */
	apt.AppendGlobalArguments("-o=quiet::NoUpdate=1")
	apt.AppendGlobalArguments("-o=Dpkg::Progress-Fancy=0")
	apt.AppendGlobalArguments("-o=Dpkg::Use-Pty=0")

	return apt
}

// Clean removes the downloaded package files.
func (apt AptCommand) Clean() error {
	return apt.Run("clean")
}

// Install installs packages, optionally with recommended packages and
// allowing unauthenticated ones.
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

// Update refreshes the package lists.
func (apt AptCommand) Update() error {
	return apt.Run("update")
}
