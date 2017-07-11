package main

import (
	"fmt"
	"github.com/sjoerdsimons/fakemachine"
	"path"
	"log"
)

type RunAction struct {
	*BaseAction
	Chroot  bool
	PostProcess  bool
	Script  string
	Command string
}

func (run *RunAction) Verify(context YaibContext) {
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
	if ! run.PostProcess {
		m.AddVolume(path.Dir(run.Script))
	}
}

func (run *RunAction) doRun(context YaibContext) {
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
			q := NewQemuHelper(context)
			q.Setup()
			defer q.Cleanup()

			options := []string{"-q", "-D", context.rootdir}
			options = append(options, "--bind", fmt.Sprintf("%s:/script",
				path.Dir(run.Script)))
			options = append(options, command)
			RunCommand(path.Base(run.Script), "systemd-nspawn", options...)
		} else {
			err = RunCommandInChroot(context, command, "sh", "-c", command)
		}
	} else {
		if ! run.PostProcess {
			command = command + " " + context.rootdir
		}
		RunCommand(path.Base(command), "sh", "-c", command)
	}

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
	if ! run.PostProcess {
		return
	}
	run.doRun(context)
}
