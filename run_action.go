package main

import (
	"fmt"
	"github.com/sjoerdsimons/fakemachine"
	"path"
)

type RunAction struct {
	*BaseAction
	Chroot  bool
	Script  string
	Command string
}

func (run *RunAction) PreMachine(context *YaibContext, m *fakemachine.Machine,
	args *[]string) {

	if run.Script == "" {
		return
	}

	run.Script = CleanPathAt(run.Script, context.recipeDir)
	m.AddVolume(path.Dir(run.Script))
}

func (run *RunAction) Run(context *YaibContext) {
	var command string

	if run.Script != "" {
		run.Script = CleanPathAt(run.Script, context.recipeDir)
		if run.Chroot {
			command = fmt.Sprintf("/script/%s", path.Base(run.Script))
		} else {
			command = run.Script
		}
	} else {
		command = run.Command
	}

	var err error
	if run.Chroot {
		if run.Script != "" {
			q := NewQemuHelper(*context)
			q.Setup()
			defer q.Cleanup()

			options := []string{"-q", "-D", context.rootdir}
			options = append(options, "--bind", fmt.Sprintf("%s:/script",
				path.Dir(run.Script)))
			options = append(options, command)
			RunCommand(path.Base(run.Script), "systemd-nspawn", options...)
		} else {
			err = RunCommandInChroot(*context, command, "sh", "-c", command)
		}
	} else {
		command = command + " " + context.rootdir
		RunCommand(path.Base(command), "sh", "-c", command)
	}

	if err != nil {
		panic(err)
	}
}
