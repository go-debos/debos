/* Base class to abstract commonly used commands. */
package wrapper

import (
	"github.com/go-debos/debos"
)

type Wrapper struct {
	debos.Command
	command    string
	globalArgs []string
	label      string
}

func NewCommandWrapper(context debos.DebosContext, command string, label string) Wrapper {
	return Wrapper{
		Command: debos.NewChrootCommandForContext(context),
		command: command,
		label:   label,
	}
}

func (cmd *Wrapper) SetCommand(command string) {
	cmd.command = command
}

func (cmd *Wrapper) AppendGlobalArguments(args string) {
	cmd.globalArgs = append(cmd.globalArgs, args)
}

func (cmd *Wrapper) SetLabel(label string) {
	cmd.label = label
}

func (cmd Wrapper) Run(additionalArgs ...string) error {
	args := []string{cmd.command}
	args = append(args, cmd.globalArgs...)
	args = append(args, additionalArgs...)

	return cmd.Command.Run(cmd.label, args...)
}
