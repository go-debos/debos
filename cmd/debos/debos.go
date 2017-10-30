package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

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

// If option BuildStorageLocation has been passed.
// Prepare the image formatted as ext4 and setup to mount it to '/scratch'
// in fake machine
func prepareBuildImage(context debos.DebosContext, buildImagePath string, buildImageSize int64) (string, error) {
	fi, err := os.Stat(buildImagePath)
	if err != nil {
		return "", err
	}
	if mode := fi.Mode(); mode.IsDir() != true {
		return "", fmt.Errorf("Location for temporary build image must have directory type.")
	}

	buildImage, err := ioutil.TempFile(buildImagePath, ".debos-build-")
	if err != nil {
		return "", err
	}

	if err := buildImage.Truncate(buildImageSize); err != nil {
		return buildImage.Name(), err
	}

	// Format the whole disk image disabling journal support
	cmdline := []string{}
	cmdline = append(cmdline, "mkfs.ext4", "-q", buildImage.Name())
	cmdline = append(cmdline, "-O", "^has_journal")
	cmd := debos.Command{}
	if err := cmd.Run(context.Scratchdir, cmdline...); err != nil {
		return buildImage.Name(), err
	}

	cmdline = []string{}
	cmdline = append(cmdline, "mount", "-t", "ext4", buildImage.Name(), context.Scratchdir, "-o", "loop")
	cmd = debos.Command{}
	if err := cmd.Run(context.Scratchdir, cmdline...); err != nil {
		return buildImage.Name(), err
	}

	return buildImage.Name(), nil
}

func main() {
	var context debos.DebosContext
	var options struct {
		ArtifactDir          string            `long:"artifactdir"`
		InternalImage        string            `long:"internal-image" hidden:"true"`
		TemplateVars         map[string]string `short:"t" long:"template-var" description:"Template variables"`
		DebugShell           bool              `long:"debug-shell" description:"Fall into interactive shell on error"`
		Shell                string            `short:"s" long:"shell" description:"Redefine interactive shell binary (default: bash)" optionsl:"" default:"/bin/bash"`
		BuildStorageLocation string            `short:"b" long:"build-storage" description:"Directory for temporary build image"`
		BuildStorageSize     string            `long:"build-storage-size" description:"The size of the temporary build image" default:"10gB"`
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
		if exitcode = checkError(context, err, a, "Verify"); exitcode != 0 {
			return
		}
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

		if len(options.BuildStorageLocation) != 0 {
			args = append(args, "--build-storage", options.BuildStorageLocation)
			args = append(args, "--build-storage-size", options.BuildStorageSize)
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
	// Prepare build image
	if len(options.BuildStorageLocation) != 0 {
		// Exit with errorcode = 1 in case of error
		exitcode = 1

		buildImageSize, err := units.FromHumanSize(options.BuildStorageSize)
		if err != nil {
			log.Println(err)
			return
		}

		blddir, err := filepath.Abs(options.BuildStorageLocation)
		if err != nil {
			log.Println(err)
			return
		}

		buildImage, err := prepareBuildImage(context, blddir, buildImageSize)
		if len(buildImage) != 0 {
			defer os.Remove(buildImage)
		}
		if err != nil {
			log.Println(err)
			return
		}

		// restore exitcode to success
		exitcode = 0
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PreNoMachine(&context)
			if exitcode = checkError(context, err, a, "PreNoMachine"); exitcode != 0 {
				return
			}
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
