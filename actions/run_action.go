package actions

import (
	"errors"
	"fmt"
	"github.com/sjoerdsimons/fakemachine"
	"path"

	"github.com/go-debos/debos"
)

type RunAction struct {
	debos.BaseAction  `yaml:",inline"`
	Chroot      bool
	PostProcess bool
	Script      string
	Command     string
}

func (run *RunAction) Verify(context *debos.DebosContext) error {
	if run.PostProcess && run.Chroot {
		return errors.New("Cannot run postprocessing in the chroot")
	}
	return nil
}

func (run *RunAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine,
	args *[]string) error {

	if run.Script == "" {
		return nil
	}

	run.Script = debos.CleanPathAt(run.Script, context.RecipeDir)
	if !run.PostProcess {
		m.AddVolume(path.Dir(run.Script))
	}

	return nil
}

func (run *RunAction) doRun(context debos.DebosContext) error {
	run.LogStart()
	var cmdline []string
	var label string
	var cmd debos.Command

	if run.Chroot {
		cmd = debos.NewChrootCommand(context.Rootdir, context.Architecture)
	} else {
		cmd = debos.Command{}
	}

	if run.Script != "" {
		run.Script = debos.CleanPathAt(run.Script, context.RecipeDir)
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
		cmd.AddEnvKey("ROOTDIR", context.Rootdir)
	}

	return cmd.Run(label, cmdline...)
}

func (run *RunAction) Run(context *debos.DebosContext) error {
	if run.PostProcess {
		/* This runs in postprocessing instead */
		return nil
	}
	return run.doRun(*context)
}

func (run *RunAction) PostMachine(context debos.DebosContext) error {
	if !run.PostProcess {
		return nil
	}
	return run.doRun(context)
}
