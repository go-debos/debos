/*
Run Action

Allows to run any available command or script in the filesystem or
in build process host environment: specifically inside the fakemachine created
by Debos.

 # Yaml syntax:
 - action: run
   chroot: bool
   postprocess: bool
   script: script name
   command: command line
   label: string

Properties 'command' and 'script' are mutually exclusive.

- command -- command with arguments; the command expected to be accessible in
host's or chrooted environment -- depending on 'chroot' property.

- script -- script with arguments; script must be located in recipe directory.

Optional properties:

- chroot -- run script or command in target filesystem if set to true.
Otherwise the command or script is executed within the build process, with
access to the filesystem ($ROOTDIR), the image if any ($IMAGE), the
recipe directory ($RECIPEDIR), the artifact directory ($ARTIFACTDIR) and the
directory where the image is mounted ($IMAGEMNTDIR).
In both cases it is run with root privileges. If unset, chroot is set to false and
the command or script is run in the host environment.

- label -- if non-empty, this string is used to label output. If empty,
a label is derived from the command or script.

- postprocess -- if set script or command is executed after all other commands and
has access to the recipe directory ($RECIPEDIR) and the artifact directory ($ARTIFACTDIR).
The working directory will be set to the artifact directory.


Properties 'chroot' and 'postprocess' are mutually exclusive.
*/
package actions

import (
	"errors"
	"github.com/go-debos/fakemachine"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

const (
	maxLabelLength = 40
)

type RunAction struct {
	debos.BaseAction `yaml:",inline"`
	Chroot           bool
	PostProcess      bool
	Script           string
	Command          string
	Label            string
}

func (run *RunAction) Verify(context *debos.DebosContext) error {
	if run.PostProcess && run.Chroot {
		return errors.New("Cannot run postprocessing in the chroot")
	}

	if run.Script == "" && run.Command == "" {
		return errors.New("Script and Command both cannot be empty")
	}
	return nil
}

func (run *RunAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine,
	args *[]string) error {

	if run.Script == "" {
		return nil
	}

	run.Script = debos.CleanPathAt(run.Script, context.RecipeDir)
	// Expect we have no blank spaces in path
	scriptpath := strings.Split(run.Script, " ")

	if !run.PostProcess {
		m.AddVolume(path.Dir(scriptpath[0]))
	}

	return nil
}

func (run *RunAction) doRun(context debos.DebosContext) error {
	var cmdline []string
	var label string
	var cmd debos.Command

	if run.Chroot {
		cmd = debos.NewChrootCommandForContext(context)
	} else {
		cmd = debos.Command{}
	}

	if run.Script != "" {
		script := strings.SplitN(run.Script, " ", 2)
		script[0] = debos.CleanPathAt(script[0], context.RecipeDir)
		if run.Chroot {
			scriptpath := path.Dir(script[0])
			cmd.AddBindMount(scriptpath, "/tmp/script")
			script[0] = strings.Replace(script[0], scriptpath, "/tmp/script", 1)
		}
		cmdline = []string{strings.Join(script, " ")}
		label = path.Base(run.Script)
	} else {
		cmdline = []string{run.Command}

		// Remove leading and trailing spaces and — importantly — newlines
		// before splitting, so that single-line scripts split into an array
		// of a single string only.
		commands := strings.Split(strings.TrimSpace(run.Command), "\n")
		label = commands[0]

		// Make it clear a long or a multi-line command is being run
		if len(label) > maxLabelLength {
			label = label[:maxLabelLength]

			label = strings.TrimSpace(label)

			label += "..."
		} else if len(commands) > 1 {
			label += "..."
		}
	}

	if run.Label != "" {
		label = run.Label
	}

	// Command/script with options passed as single string
	cmdline = append([]string{"sh", "-c"}, cmdline...)

	if !run.Chroot {
		cmd.AddEnvKey("RECIPEDIR", context.RecipeDir)
		cmd.AddEnvKey("ARTIFACTDIR", context.Artifactdir)
	}

	if !run.PostProcess {
		if !run.Chroot {
			cmd.AddEnvKey("ROOTDIR", context.Rootdir)
			if context.ImageMntDir != "" {
				cmd.AddEnvKey("IMAGEMNTDIR", context.ImageMntDir)
			}
		}
		if context.Image != "" {
			cmd.AddEnvKey("IMAGE", context.Image)
		}
	}

	/* For PostProcess commands, set cwd to artifactdir */
	if run.PostProcess {
		cmd.Dir = context.Artifactdir
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

func (run *RunAction) PostMachine(context *debos.DebosContext) error {
	if !run.PostProcess {
		return nil
	}
	return run.doRun(*context)
}
