/* Base class to abstract commonly used commands. */
package wrapper

import (
	"fmt"
	"github.com/go-debos/debos"
)

type Wrapper struct {
	debos.Command
	command    string
	globalArgs []string
	label      string
}

func NewCommandWrapper(context debos.Context, command string, label string) Wrapper {
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

	if err := cmd.Command.Run(cmd.label, args...); err != nil {
		return fmt.Errorf("%s: %w", cmd.label, err)
	}
	return nil
}
