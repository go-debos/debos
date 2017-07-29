package main

import (
	"fmt"
	"github.com/sjoerdsimons/fakemachine"
	"log"
	"path"
)

type RunAction struct {
	*BaseAction
	Chroot      bool
	PostProcess bool
	Script      string
	Command     string
}

func (run *RunAction) Verify(context *YaibContext) {
	if run.PostProcess && run.Chroot {
		log.Fatal("Cannot use both chroot and postprocess in a run action")
	}
}

func (run *RunAction) PreMachine(context *YaibContext, m *fakemachine.Machine,
	args *[]string) {

	if run.Script == "" {
		return
	}

	run.Script = CleanPathAt(run.Script, context.recipeDir)
	if !run.PostProcess {
		m.AddVolume(path.Dir(run.Script))
	}
}

func (run *RunAction) doRun(context YaibContext) {
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
		cmdline = append(cmdline, context.rootdir)
	}

	err := cmd.Run(label, cmdline...)

	if err != nil {
		panic(err)
	}
}

func (run *RunAction) Run(context *YaibContext) {
	if run.PostProcess {
		/* This runs in postprocessing instead */
		return
	}
	run.doRun(*context)
}

func (run *RunAction) PostMachine(context YaibContext) {
	if !run.PostProcess {
		return
	}
	run.doRun(context)
}
