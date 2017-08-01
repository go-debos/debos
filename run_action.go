package main

import (
	"errors"
	"fmt"
	"github.com/sjoerdsimons/fakemachine"
	"path"
)

type RunAction struct {
	BaseAction  `yaml:",inline"`
	Chroot      bool
	PostProcess bool
	Script      string
	Command     string
}

func (run *RunAction) Verify(context *YaibContext) error {
	if run.PostProcess && run.Chroot {
		return errors.New("Cannot use both chroot and postprocess in a run action")
	}
	return nil
}

func (run *RunAction) PreMachine(context *YaibContext, m *fakemachine.Machine,
	args *[]string) error {

	if run.Script == "" {
		return nil
	}

	run.Script = CleanPathAt(run.Script, context.recipeDir)
	if !run.PostProcess {
		m.AddVolume(path.Dir(run.Script))
	}

	return nil
}

func (run *RunAction) doRun(context YaibContext) error {
	var cmdline []string
	var label string
	var cmd Command

	if run.Chroot {
		cmd = NewChrootCommand(context.rootdir, context.Architecture)
	} else {
		cmd = Command{}
	}

	if run.Script != "" {
		run.Script = CleanPathAt(run.Script, context.recipeDir)
		if run.Chroot {
			cmd.AddBindMount(path.Dir(run.Script), "/script")
			cmdline = []string{fmt.Sprintf("/script/%s", path.Base(run.Script))}
		} else {
			cmdline = []string{run.Script}
		}
		label = path.Base(run.Script)
	} else {
		cmdline = []string{"sh", "-c", run.Command}
		label = run.Command
	}

	if !run.Chroot && !run.PostProcess {
		cmd.AddEnvKey("ROOTDIR", context.rootdir)
	}

	return cmd.Run(label, cmdline...)
}

func (run *RunAction) Run(context *YaibContext) error {
	if run.PostProcess {
		/* This runs in postprocessing instead */
		return nil
	}
	return run.doRun(*context)
}

func (run *RunAction) PostMachine(context YaibContext) error {
	if !run.PostProcess {
		return nil
	}
	return run.doRun(context)
}
