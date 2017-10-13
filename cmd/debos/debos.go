package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/recipe"
	"github.com/jessevdk/go-flags"
	"github.com/sjoerdsimons/fakemachine"
)

func bailOnError(context debos.DebosContext, err error, a debos.Action, stage string) {
	if err == nil {
		return
	}

	log.Printf("Action `%s` failed at stage %s, error: %s", a, stage, err)
	debos.DebugShell(context)
	os.Exit(1)
}

func main() {
	var context debos.DebosContext
	var options struct {
		ArtifactDir   string            `long:"artifactdir"`
		InternalImage string            `long:"internal-image" hidden:"true"`
		TemplateVars  map[string]string `short:"t" long:"template-var" description:"Template variables"`
		DebugShell    bool              `long:"debug-shell" description:"Fall into interactive shell on error"`
		Shell         string            `short:"s" long:"shell" description:"Redefine interactive shell binary (default: bash)" optionsl:"" default:"/bin/bash"`
	}

	var exitcode int = 0
	// Allow to run all deferred calls prior to os.Exit()
	defer func() {
		os.Exit(exitcode)
	}()

	parser := flags.NewParser(&options, flags.Default)
	args, err := parser.Parse()

	if err != nil {
		flagsErr, ok := err.(*flags.Error)
		if ok && flagsErr.Type == flags.ErrHelp {
			return
		} else {
			fmt.Printf("%v\n", flagsErr)
			exitcode = 1
			return
		}
	}

	if len(args) != 1 {
		log.Println("No recipe given!")
		exitcode = 1
		return
	}

	// Set interactive shell binary only if '--debug-shell' options passed
	if options.DebugShell {
		context.DebugShell = options.Shell
	}

	file := args[0]
	file = debos.CleanPath(file)

	r := recipe.Recipe{}
	if err := r.Parse(file, options.TemplateVars); err != nil {
		panic(err)
	}

	/* If fakemachine is supported the outer fake machine will never use the
	 * scratchdir, so just set it to /scrach as a dummy to prevent the outer
	 * debos createing a temporary direction */
	if fakemachine.InMachine() || fakemachine.Supported() {
		context.Scratchdir = "/scratch"
	} else {
		log.Printf("fakemachine not supported, running on the host!")
		cwd, _ := os.Getwd()
		context.Scratchdir, err = ioutil.TempDir(cwd, ".debos-")
		defer os.RemoveAll(context.Scratchdir)
	}

	context.Rootdir = path.Join(context.Scratchdir, "root")
	context.Image = options.InternalImage
	context.RecipeDir = path.Dir(file)

	context.Artifactdir = options.ArtifactDir
	if context.Artifactdir == "" {
		context.Artifactdir, _ = os.Getwd()
	}
	context.Artifactdir = debos.CleanPath(context.Artifactdir)

	// Initialise origins map
	context.Origins = make(map[string]string)
	context.Origins["artifacts"] = context.Artifactdir
	context.Origins["filesystem"] = context.Rootdir
	context.Origins["recipe"] = context.RecipeDir

	context.Architecture = r.Architecture

	for _, a := range r.Actions {
		err = a.Verify(&context)
		bailOnError(context, err, a, "Verify")
	}

	if !fakemachine.InMachine() && fakemachine.Supported() {
		m := fakemachine.NewMachine()
		var args []string

		m.AddVolume(context.Artifactdir)
		args = append(args, "--artifactdir", context.Artifactdir)

		for k, v := range options.TemplateVars {
			args = append(args, "--template-var", fmt.Sprintf("%s:\"%s\"", k, v))
		}

		m.AddVolume(context.RecipeDir)
		args = append(args, file)

		if options.DebugShell {
			args = append(args, "--debug-shell")
			args = append(args, "--shell", fmt.Sprintf("%s", options.Shell))
		}

		for _, a := range r.Actions {
			err = a.PreMachine(&context, m, &args)
			bailOnError(context, err, a, "PreMachine")
		}

		exitcode = m.RunInMachineWithArgs(args)

		if exitcode != 0 {
			return
		}

		for _, a := range r.Actions {
			err = a.PostMachine(context)
			bailOnError(context, err, a, "Postmachine")
		}

		log.Printf("==== Recipe done ====")
		return
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PreNoMachine(&context)
			bailOnError(context, err, a, "PreNoMachine")
		}
	}

	for _, a := range r.Actions {
		err = a.Run(&context)

		bailOnError(context, err, a, "Run")
	}

	for _, a := range r.Actions {
		err = a.Cleanup(context)
		bailOnError(context, err, a, "Cleanup")
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PostMachine(context)
			bailOnError(context, err, a, "PostMachine")
		}
		log.Printf("==== Recipe done ====")
	}
}
