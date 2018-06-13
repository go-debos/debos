package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/docker/go-units"
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/recipe"
	"github.com/go-debos/fakemachine"
	"github.com/jessevdk/go-flags"
)

func checkError(context debos.DebosContext, err error, a debos.Action, stage string) int {
	if err == nil {
		return 0
	}

	log.Printf("Action `%s` failed at stage %s, error: %s", a, stage, err)
	debos.DebugShell(context)
	return 1
}

func main() {
	var context debos.DebosContext
	var options struct {
		ArtifactDir   string            `long:"artifactdir" description:"Directory for packed archives and ostree repositories (default: current directory)"`
		InternalImage string            `long:"internal-image" hidden:"true"`
		TemplateVars  map[string]string `short:"t" long:"template-var" description:"Template variables (use -t VARIABLE:VALUE syntax)"`
		DebugShell    bool              `long:"debug-shell" description:"Fall into interactive shell on error"`
		Shell         string            `short:"s" long:"shell" description:"Redefine interactive shell binary (default: bash)" optionsl:"" default:"/bin/bash"`
		ScratchSize   string            `long:"scratchsize" description:"Size of disk backed scratch space"`
		CPUs          int               `short:"c" long:"cpus" description:"Number of CPUs to use for build VM (default: 2)"`
		Memory        string            `short:"m" long:"memory" description:"Amount of memory for build VM (default: 2048MB)"`
		ShowBoot      bool              `long:"show-boot" description:"Show boot/console messages from the fake machine"`
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
	if _, err := os.Stat(file); os.IsNotExist(err) {
		log.Println(err)
		exitcode = 1
		return
	}
	if err := r.Parse(file, options.TemplateVars); err != nil {
		log.Println(err)
		exitcode = 1
		return
	}

	/* If fakemachine is supported the outer fake machine will never use the
	 * scratchdir, so just set it to /scratch as a dummy to prevent the
	 * outer debos creating a temporary direction */
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
		if exitcode = checkError(context, err, a, "Verify"); exitcode != 0 {
			return
		}
	}

	if !fakemachine.InMachine() && fakemachine.Supported() {
		m := fakemachine.NewMachine()
		var args []string

		if options.Memory == "" {
			// Set default memory size for fakemachine
			options.Memory = "2Gb"
		}
		memsize, err := units.RAMInBytes(options.Memory)
		if err != nil {
			fmt.Printf("Couldn't parse memory size: %v\n", err)
			exitcode = 1
			return
		}
		m.SetMemory(int(memsize / 1024 / 1024))

		if options.CPUs == 0 {
			// Set default CPU count for fakemachine
			options.CPUs = 2
		}
		m.SetNumCPUs(options.CPUs)

		if options.ScratchSize != "" {
			size, err := units.FromHumanSize(options.ScratchSize)
			if err != nil {
				fmt.Printf("Couldn't parse scratch size: %v\n", err)
				exitcode = 1
				return
			}
			m.SetScratch(size, "")
		}

		m.SetShowBoot(options.ShowBoot)

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
			if exitcode = checkError(context, err, a, "PreMachine"); exitcode != 0 {
				return
			}
		}

		exitcode, err = m.RunInMachineWithArgs(args)
		if err != nil {
			fmt.Println(err)
			return
		}

		if exitcode != 0 {
			return
		}

		for _, a := range r.Actions {
			err = a.PostMachine(context)
			if exitcode = checkError(context, err, a, "Postmachine"); exitcode != 0 {
				return
			}
		}

		log.Printf("==== Recipe done ====")
		return
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PreNoMachine(&context)
			if exitcode = checkError(context, err, a, "PreNoMachine"); exitcode != 0 {
				return
			}
		}
	}

	// Create Rootdir
	if _, err = os.Stat(context.Rootdir); os.IsNotExist(err) {
		err = os.Mkdir(context.Rootdir, 0755)
		if err != nil && os.IsNotExist(err) {
			exitcode = 1
			return
		}
	}

	for _, a := range r.Actions {
		err = a.Run(&context)
		if exitcode = checkError(context, err, a, "Run"); exitcode != 0 {
			return
		}
	}

	for _, a := range r.Actions {
		err = a.Cleanup(context)
		if exitcode = checkError(context, err, a, "Cleanup"); exitcode != 0 {
			return
		}
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PostMachine(context)
			if exitcode = checkError(context, err, a, "PostMachine"); exitcode != 0 {
				return
			}
		}
		log.Printf("==== Recipe done ====")
	}
}
