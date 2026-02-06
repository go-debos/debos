package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime/debug"
	"strings"

	"github.com/docker/go-units"
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/go-debos/fakemachine"
	"github.com/jessevdk/go-flags"
)

var Version string

func GetDeterminedVersion(version string) string {
	DeterminedVersion := "unknown"

	// Use the injected Version from build system if any.
	// Otherwise try to determine the best version string from debug info.
	if len(version) > 0 {
		DeterminedVersion = version
	} else {
		info, ok := debug.ReadBuildInfo()
		if ok {
			// Try vcs version first as it will only be set on a local build
			var revision *string
			var modified *string
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					revision = &s.Value
				}
				if s.Key == "vcs.modified" {
					modified = &s.Value
				}
			}
			if revision != nil {
				DeterminedVersion = *revision
				if modified != nil && *modified == "true" {
					DeterminedVersion += "-dirty"
				}
			} else {
				DeterminedVersion = info.Main.Version
			}
		}
	}

	return DeterminedVersion
}

func handleError(context *debos.Context, err error, a debos.Action, stage string) bool {
	if err == nil {
		return false
	}

	context.State = debos.Failed
	log.Printf("Action `%s` failed at stage %s, error: %s", a, stage, err)
	debos.DebugShell(*context)
	return true
}

func doRun(r actions.Recipe, context *debos.Context) bool {
	for _, a := range r.Actions {
		log.Printf("==== %s ====\n", a)
		err := a.Run(context)

		// This does not stop the call of stacked Cleanup methods for other Actions
		// Stack Cleanup methods
		defer func(action debos.Action) {
			_ = action.Cleanup(context)
		}(a)

		// Check the state of Run method
		if handleError(context, err, a, "Run") {
			return false
		}
	}

	return true
}

func warnLocalhost(variable string, value string) {
	message := `WARNING: Environment variable %[1]s contains a reference to
		    localhost. This may not work when running from fakemachine.
		    Consider using an address that is valid on your network.`

	if strings.Contains(value, "localhost") ||
		strings.Contains(value, "127.0.0.1") ||
		strings.Contains(value, "::1") {
		log.Printf(message, variable)
	}
}

func main() {
	context := debos.Context{
		CommonContext: &debos.CommonContext{},
		RecipeDir:     "",
		Architecture:  "",
		SectorSize:    512,
	}
	var options struct {
		Backend            string            `short:"b" long:"fakemachine-backend" description:"Fakemachine backend to use" default:"auto"`
		ArtifactDir        string            `long:"artifactdir" description:"Directory for packed archives and ostree repositories (default: current directory)"`
		InternalImage      string            `long:"internal-image" hidden:"true"`
		TemplateVars       map[string]string `short:"t" long:"template-var" description:"Template variables (use -t VARIABLE:VALUE syntax)"`
		DebugShell         bool              `long:"debug-shell" description:"Fall into interactive shell on error"`
		Shell              string            `short:"s" long:"shell" description:"Redefine interactive shell binary (default: bash)" optionsl:"" default:"/bin/bash"`
		ScratchSize        string            `long:"scratchsize" description:"Size of disk-backed scratch space (parsed with human-readable suffix; assumed bytes if no suffix)"`
		CPUs               int               `short:"c" long:"cpus" description:"Number of CPUs to use for build VM (default: 2)"`
		Memory             string            `short:"m" long:"memory" description:"Amount of memory for build VM (parsed with human-readable suffix; assumed bytes if no suffix. default: 2Gb)"`
		ShowBoot           bool              `long:"show-boot" description:"Show boot/console messages from the fake machine"`
		EnvironVars        map[string]string `short:"e" long:"environ-var" description:"Environment variables (use -e VARIABLE:VALUE syntax)"`
		Verbose            bool              `short:"v" long:"verbose" description:"Verbose output"`
		PrintRecipe        bool              `long:"print-recipe" description:"Print final recipe"`
		DryRun             bool              `long:"dry-run" description:"Compose final recipe to build but without any real work started"`
		DisableFakeMachine bool              `long:"disable-fakemachine" description:"Do not use fakemachine."`
		Version            bool              `long:"version" description:"Print debos version"`
	}

	// These are the environment variables that will be detected on the
	// host and propagated to fakemachine. These are listed lower case, but
	// they are detected and configured in both lower case and upper case.
	var environVars = [...]string{
		"http_proxy",
		"https_proxy",
		"ftp_proxy",
		"rsync_proxy",
		"all_proxy",
		"no_proxy",
	}

	// Allow to run all deferred calls prior to os.Exit()
	defer func(context debos.Context) {
		if context.State == debos.Failed {
			os.Exit(1)
		}
	}(context)

	parser := flags.NewParser(&options, flags.Default)
	fakemachineBackends := parser.FindOptionByLongName("fakemachine-backend")
	fakemachineBackends.Choices = fakemachine.BackendNames()

	args, err := parser.Parse()
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return
		}
		context.State = debos.Failed
		return
	}

	if options.Version {
		fmt.Printf("debos %v\n", GetDeterminedVersion(Version))
		return
	}

	if len(args) != 1 {
		log.Println("No recipe given!")
		context.State = debos.Failed
		return
	}

	if options.DisableFakeMachine && options.Backend != "auto" {
		log.Println("--disable-fakemachine and --fakemachine-backend are mutually exclusive")
		context.State = debos.Failed
		return
	}

	// Set interactive shell binary only if '--debug-shell' options passed
	if options.DebugShell {
		context.DebugShell = options.Shell
	}

	if options.PrintRecipe {
		context.PrintRecipe = options.PrintRecipe
	}

	if options.Verbose {
		context.Verbose = options.Verbose
	}

	file := args[0]
	file = debos.CleanPath(file)

	r := actions.Recipe{}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		log.Println(err)
		context.State = debos.Failed
		return
	}
	if err := r.Parse(file, options.PrintRecipe, options.Verbose, options.TemplateVars); err != nil {
		// err contains multiple lines - log them individually to retain timestamp
		log.Println("Recipe parsing failed:")
		for _, line := range strings.Split(strings.TrimRight(err.Error(), "\n"), "\n") {
			log.Printf("%s", line)
		}

		context.State = debos.Failed
		return
	}

	/* If fakemachine is used the outer fake machine will never use the
	 * scratchdir, so just set it to /scratch as a dummy to prevent the
	 * outer debos creating a temporary directory */
	context.Scratchdir = "/scratch"

	var runInFakeMachine = true
	var m *fakemachine.Machine
	if options.DisableFakeMachine || fakemachine.InMachine() {
		runInFakeMachine = false
	} else {
		// attempt to create a fakemachine
		m, err = fakemachine.NewMachineWithBackend(options.Backend)
		if err != nil {
			log.Printf("Couldn't create fakemachine: %v", err)

			/* fallback to running on the host unless the user has chosen
			 * a specific backend */
			if options.Backend == "auto" {
				runInFakeMachine = false
			} else {
				context.State = debos.Failed
				return
			}
		}
	}

	// if running on the host create a scratchdir
	if !runInFakeMachine && !fakemachine.InMachine() {
		log.Printf("fakemachine not supported, running on the host!")
		cwd, _ := os.Getwd()
		context.Scratchdir, _ = os.MkdirTemp(cwd, ".debos-")
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
	if dirInfo, err := os.Stat(context.Artifactdir); err != nil || !dirInfo.IsDir() {
		log.Printf("Artifact Directory %s does not exist or is not a directory\n", context.Artifactdir)
		context.State = debos.Failed
		return
	}

	// Initialise origins map
	context.Origins = make(map[string]string)
	context.Origins["artifacts"] = context.Artifactdir
	context.Origins["filesystem"] = context.Rootdir
	context.Origins["recipe"] = context.RecipeDir

	context.Architecture = r.Architecture
	context.SectorSize = r.SectorSize

	context.State = debos.Success

	// Initialize environment variables map
	context.EnvironVars = make(map[string]string)

	// First add variables from host
	for _, e := range environVars {
		lowerVar := strings.ToLower(e) // lowercase not really needed
		lowerVal := os.Getenv(lowerVar)
		if lowerVal != "" {
			context.EnvironVars[lowerVar] = lowerVal
		}

		upperVar := strings.ToUpper(e)
		upperVal := os.Getenv(upperVar)
		if upperVal != "" {
			context.EnvironVars[upperVar] = upperVal
		}
	}

	// Then add/overwrite with variables from command line
	for k, v := range options.EnvironVars {
		// Allows the user to unset environ variables with -e
		if v == "" {
			delete(context.EnvironVars, k)
		} else {
			context.EnvironVars[k] = v
		}
	}

	for _, a := range r.Actions {
		err = a.Verify(&context)
		if handleError(&context, err, a, "Verify") {
			return
		}
	}

	if options.DryRun {
		log.Printf("==== Recipe done (Dry run) ====")
		return
	}

	if runInFakeMachine {
		var args []string

		if options.Memory == "" {
			// Set default memory size for fakemachine
			options.Memory = "2Gb"
		}
		memsize, err := units.RAMInBytes(options.Memory)
		if err != nil {
			log.Printf("Couldn't parse memory size: %v\n", err)
			context.State = debos.Failed
			return
		}

		memsizeMB := int(memsize / 1024 / 1024)
		if memsizeMB < 256 {
			log.Printf("WARNING: Memory size of %dMB is less than recommended minimum 256MB\n", memsizeMB)
		}
		m.SetMemory(memsizeMB)

		if options.CPUs == 0 {
			// Set default CPU count for fakemachine
			options.CPUs = 2
		}
		m.SetNumCPUs(options.CPUs)
		m.SetSectorSize(r.SectorSize)

		if options.ScratchSize != "" {
			size, err := units.FromHumanSize(options.ScratchSize)
			if err != nil {
				log.Printf("Couldn't parse scratch size: %v\n", err)
				context.State = debos.Failed
				return
			}

			scratchsizeMB := int(size / 1000 / 1000)
			if scratchsizeMB < 512 {
				log.Printf("WARNING: Scratch size of %dMB is less than recommended minimum 512MB\n", scratchsizeMB)
			}
			m.SetScratch(size, "")
		}

		m.SetShowBoot(options.ShowBoot)

		// Puts in a format that is compatible with output of os.Environ()
		if context.EnvironVars != nil {
			EnvironString := []string{}
			for k, v := range context.EnvironVars {
				warnLocalhost(k, v)
				EnvironString = append(EnvironString, fmt.Sprintf("%s=%s", k, v))
			}
			m.SetEnviron(EnvironString) // And save the resulting environ vars on m
		}

		m.AddVolume(context.Artifactdir)
		args = append(args, "--artifactdir", context.Artifactdir)

		for k, v := range options.TemplateVars {
			args = append(args, "--template-var", fmt.Sprintf("%s:%s", k, v))
		}

		for k, v := range options.EnvironVars {
			args = append(args, "--environ-var", fmt.Sprintf("%s:%s", k, v))
		}

		m.AddVolume(context.RecipeDir)
		args = append(args, file)

		if options.DebugShell {
			args = append(args, "--debug-shell")
			args = append(args, "--shell", options.Shell)
		}

		if options.Verbose {
			args = append(args, "--verbose")
		}

		for _, a := range r.Actions {
			// Stack PostMachineCleanup methods
			defer func(action debos.Action) {
				_ = action.PostMachineCleanup(&context)
			}(a)

			err = a.PreMachine(&context, m, &args)
			if handleError(&context, err, a, "PreMachine") {
				return
			}
		}

		// Silence extra output from fakemachine unless the --verbose flag was passed.
		m.SetQuiet(!options.Verbose)

		exitcode, err := m.RunInMachineWithArgs(args)
		if err != nil {
			log.Printf("Couldn't start fakemachine: %v\n", err)
			context.State = debos.Failed
			return
		}

		if exitcode != 0 {
			log.Printf("fakemachine failed with non-zero exitcode: %d\n", exitcode)
			context.State = debos.Failed
			return
		}

		for _, a := range r.Actions {
			err = a.PostMachine(&context)
			if handleError(&context, err, a, "PostMachine") {
				return
			}
		}

		log.Printf("==== Recipe done ====")
		return
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			// Stack PostMachineCleanup methods
			defer func(action debos.Action) {
				_ = action.PostMachineCleanup(&context)
			}(a)

			err = a.PreNoMachine(&context)
			if handleError(&context, err, a, "PreNoMachine") {
				return
			}
		}
	}

	// Create Rootdir
	if _, err = os.Stat(context.Rootdir); os.IsNotExist(err) {
		err = os.Mkdir(context.Rootdir, 0755)
		if err != nil && os.IsNotExist(err) {
			log.Printf("Couldn't create rootdir: %v\n", err)
			context.State = debos.Failed
			return
		}
	}

	if !doRun(r, &context) {
		return
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PostMachine(&context)
			if handleError(&context, err, a, "PostMachine") {
				return
			}
		}
		log.Printf("==== Recipe done ====")
	}
}
