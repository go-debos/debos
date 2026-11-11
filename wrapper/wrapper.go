// Package wrapper abstracts commonly used commands, such as apt, so that
// actions can run them in the chroot of a recipe.
package wrapper

import (
	"fmt"

	"github.com/go-debos/debos"
)

// Wrapper runs a fixed command with global arguments, in the chroot of the
// context.
type Wrapper struct {
	debos.Command

	command    string
	globalArgs []string
	label      string
}

// NewCommandWrapper returns a Wrapper for command, logging with label.
func NewCommandWrapper(context debos.Context, command string, label string) Wrapper {
	return Wrapper{
		Command: debos.NewChrootCommandForContext(context),
		command: command,
		label:   label,
	}
}

// SetCommand sets the command to run.
func (cmd *Wrapper) SetCommand(command string) {
	cmd.command = command
}

// AppendGlobalArguments adds args after the command on every Run.
func (cmd *Wrapper) AppendGlobalArguments(args string) {
	cmd.globalArgs = append(cmd.globalArgs, args)
}

// SetLabel sets the label used when logging the output of the command.
func (cmd *Wrapper) SetLabel(label string) {
	cmd.label = label
}

// Run runs the command with the global arguments followed by additionalArgs.
func (cmd *Wrapper) Run(additionalArgs ...string) error {
	args := []string{cmd.command}
	args = append(args, cmd.globalArgs...)
	args = append(args, additionalArgs...)

	if err := cmd.Command.Run(cmd.label, args...); err != nil {
		return fmt.Errorf("%s: %w", cmd.label, err)
	}

	return nil
}
