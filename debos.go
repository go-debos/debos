package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/jessevdk/go-flags"
	"github.com/sjoerdsimons/fakemachine"

	"gopkg.in/yaml.v2"
)

func CleanPathAt(path, at string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	return filepath.Join(at, path)
}

func CleanPath(path string) string {
	cwd, _ := os.Getwd()
	return CleanPathAt(path, cwd)
}

func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func CopyTree(sourcetree, desttree string) error {
	fmt.Printf("Overlaying %s on %s\n", sourcetree, desttree)
	walker := func(p string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		suffix, _ := filepath.Rel(sourcetree, p)
		target := path.Join(desttree, suffix)
		switch info.Mode() & os.ModeType {
		case 0:
			CopyFile(p, target, info.Mode())
		case os.ModeDir:
			os.Mkdir(target, info.Mode())
		case os.ModeSymlink:
			link, err := os.Readlink(p)
			if err != nil {
				log.Panic("Failed to read symlink %s: %v", suffix, err)
			}
			os.Symlink(link, target)
		default:
			log.Panicf("Not handled /%s %v", suffix, info.Mode())
		}

		return nil
	}

	return filepath.Walk(sourcetree, walker)
}

type DebosContext struct {
	scratchdir      string
	rootdir         string
	artifactdir     string
	image           string
	imageMntDir     string
	imageFSTab      bytes.Buffer // Fstab as per partitioning
	imageKernelRoot string       // Kernel cmdline root= snippet for the / of the image
	recipeDir       string
	Architecture    string
	origins         map[string]string
}

type Action interface {
	/* FIXME verify should probably be prepare or somesuch */
	Verify(context *DebosContext) error
	PreMachine(context *DebosContext, m *fakemachine.Machine, args *[]string) error
	PreNoMachine(context *DebosContext) error
	Run(context *DebosContext) error
	Cleanup(context DebosContext) error
	PostMachine(context DebosContext) error
	String() string
}

type BaseAction struct {
	Action      string
	Description string
}

func (b *BaseAction) LogStart() {
	log.Printf("==== %s ====\n", b)
}

func (b *BaseAction) Verify(context *DebosContext) error { return nil }
func (b *BaseAction) PreMachine(context *DebosContext,
	m *fakemachine.Machine,
	args *[]string) error {
	return nil
}
func (b *BaseAction) PreNoMachine(context *DebosContext) error { return nil }
func (b *BaseAction) Run(context *DebosContext) error          { return nil }
func (b *BaseAction) Cleanup(context DebosContext) error       { return nil }
func (b *BaseAction) PostMachine(context DebosContext) error   { return nil }
func (b *BaseAction) String() string {
	if b.Description == "" {
		return b.Action
	}
	return b.Description
}

/* the YamlAction just embed the Action interface and implements the
 * UnmarshalYAML function so it can select the concrete implementer of a
 * specific action at unmarshaling time */
type YamlAction struct {
	Action
}

func (y *YamlAction) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux BaseAction

	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	switch aux.Action {
	case "debootstrap":
		y.Action = &DebootstrapAction{}
	case "pack":
		y.Action = &PackAction{}
	case "unpack":
		y.Action = &UnpackAction{}
	case "run":
		y.Action = &RunAction{}
	case "apt":
		y.Action = &AptAction{}
	case "ostree-commit":
		y.Action = &OstreeCommitAction{}
	case "ostree-deploy":
		y.Action = newOstreeDeployAction()
	case "overlay":
		y.Action = &OverlayAction{}
	case "image-partition":
		y.Action = &ImagePartitionAction{}
	case "filesystem-deploy":
		y.Action = newFilesystemDeployAction()
	case "raw":
		y.Action = &RawAction{}
	case "download":
		y.Action = &DownloadAction{}
	default:
		log.Fatalf("Unknown action: %v", aux.Action)
	}

	unmarshal(y.Action)

	return nil
}

func sector(s int) int {
	return s * 512
}

type Recipe struct {
	Architecture string
	Actions      []YamlAction
}

func bailOnError(err error, a Action, stage string) {
	if err == nil {
		return
	}

	log.Fatalf("Action `%s` failed at stage %s, error: %s", a, stage, err)
}

func main() {
	var context DebosContext
	var options struct {
		ArtifactDir   string            `long:"artifactdir"`
		InternalImage string            `long:"internal-image" hidden:"true"`
		TemplateVars  map[string]string `short:"t" long:"template-var" description:"Template variables"`
	}

	parser := flags.NewParser(&options, flags.Default)
	args, err := parser.Parse()

	if err != nil {
		flagsErr, ok := err.(*flags.Error)
		if ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Printf("%v\n", flagsErr)
			os.Exit(1)
		}
	}

	if len(args) != 1 {
		log.Fatal("No recipe given!")
	}

	file := args[0]
	file = CleanPath(file)

	/* If fakemachine is supported the outer fake machine will never use the
	 * scratchdir, so just set it to /scrach as a dummy to prevent the outer
	 * debos createing a temporary direction */
	if fakemachine.InMachine() || fakemachine.Supported() {
		context.scratchdir = "/scratch"
	} else {
		log.Printf("fakemachine not supported, running on the host!")
		cwd, _ := os.Getwd()
		context.scratchdir, err = ioutil.TempDir(cwd, ".debos-")
		defer os.RemoveAll(context.scratchdir)
	}

	context.rootdir = path.Join(context.scratchdir, "root")
	context.image = options.InternalImage
	context.recipeDir = path.Dir(file)

	context.artifactdir = options.ArtifactDir
	if context.artifactdir == "" {
		context.artifactdir, _ = os.Getwd()
	}
	context.artifactdir = CleanPath(context.artifactdir)

	t := template.New(path.Base(file))
	funcs := template.FuncMap{
		"sector": sector,
	}
	t.Funcs(funcs)

	_, err = t.ParseFiles(file)
	if err != nil {
		panic(err)
	}

	data := new(bytes.Buffer)
	err = t.Execute(data, options.TemplateVars)
	if err != nil {
		panic(err)
	}

	// Initialise origins map
	context.origins = make(map[string]string)
	context.origins["artifacts"] = context.artifactdir
	context.origins["filesystem"] = context.rootdir
	context.origins["recipe"] = context.recipeDir

	r := Recipe{}

	err = yaml.Unmarshal(data.Bytes(), &r)
	if err != nil {
		panic(err)
	}

	context.Architecture = r.Architecture

	for _, a := range r.Actions {
		err = a.Verify(&context)
		bailOnError(err, a, "Verify")
	}

	if !fakemachine.InMachine() && fakemachine.Supported() {
		m := fakemachine.NewMachine()
		var args []string

		m.AddVolume(context.artifactdir)
		args = append(args, "--artifactdir", context.artifactdir)

		for k, v := range options.TemplateVars {
			args = append(args, "--template-var", fmt.Sprintf("%s:\"%s\"", k, v))
		}

		m.AddVolume(context.recipeDir)
		args = append(args, file)

		for _, a := range r.Actions {
			err = a.PreMachine(&context, m, &args)
			bailOnError(err, a, "PreMachine")
		}

		ret := m.RunInMachineWithArgs(args)

		if ret != 0 {
			os.Exit(ret)
		}

		for _, a := range r.Actions {
			err = a.PostMachine(context)
			bailOnError(err, a, "Postmachine")
		}

		log.Printf("==== Recipe done ====")
		os.Exit(0)
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PreNoMachine(&context)
			bailOnError(err, a, "PreNoMachine")
		}
	}

	for _, a := range r.Actions {
		err = a.Run(&context)
		bailOnError(err, a, "Run")
	}

	for _, a := range r.Actions {
		err = a.Cleanup(context)
		bailOnError(err, a, "Cleanup")
	}

	if !fakemachine.InMachine() {
		for _, a := range r.Actions {
			err = a.PostMachine(context)
			bailOnError(err, a, "PostMachine")
		}
		log.Printf("==== Recipe done ====")
	}
}
