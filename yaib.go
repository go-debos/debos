package main

import (
  "flag"
  "io"
  "bufio"
  "fmt"
  "io/ioutil"
  "os"
  "os/exec"
  "path"
  "path/filepath"

  "github.com/sjoerdsimons/fakemachine"

  "gopkg.in/yaml.v2"
)

func CleanPath(path string) string {
  if filepath.IsAbs(path) {
    return filepath.Clean(path)
  }

  cwd, _ := os.Getwd()
  return filepath.Join(cwd, path)
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

func CopyTree(sourcetree, desttree string) {
  fmt.Printf("Overlaying %s on %s\n", sourcetree, desttree)
  walker := func(p string, info os.FileInfo, err error) error {

    suffix, _ := filepath.Rel(sourcetree, p)
    target := path.Join(desttree, suffix)
    if info.Mode().IsDir() {
       fmt.Printf("D> %s -> %s\n", p, target)
       os.Mkdir(target, info.Mode())
    } else if info.Mode().IsRegular() {
       fmt.Printf("F> %s\n", p)
    } else {
      panic("Not handled")
    }

    return nil
  }

  filepath.Walk(sourcetree, walker)
}

func RunCommand(name string, arg ...string) error {
  cmd := exec.Command(name, arg...)

  output, _ := cmd.StdoutPipe()
  stderr, _ := cmd.StderrPipe()

  cmd.Start()

  scanner := bufio.NewScanner(output)
  for scanner.Scan() {
    fmt.Printf("%s O | %s\n", name, scanner.Text())
  }


  reader := bufio.NewReader(stderr)

  for {
       line, _, err := reader.ReadLine()

      if (err == io.EOF) {
        fmt.Printf("EOF\n")
         break
      } else if err != nil {
        fmt.Printf("FAILED: %v\n", err)
        break;
      }
      fmt.Printf("%s E | %s\n", name, line)
  }

  err := cmd.Wait()

  return err
}

type YaibContext struct {
  rootdir string
  artifactdir string
  image string
}

type Action interface {
  Run(context YaibContext);
}

type Recipe struct {
  Actions yaml.MapSlice
}

type PackFilesystemAction struct {
  format string;
  target string;
}

func NewPackFilesystemAction(p map[string]interface{}) *PackFilesystemAction {
  pf := new(PackFilesystemAction)
  pf.target = p["target"].(string)
  pf.format = p["format"].(string)
  return pf
}

func (pf *PackFilesystemAction) Run(context YaibContext) {
  outfile := path.Join(context.artifactdir, pf.target)

  fmt.Printf("Compression to %s\n", outfile)
  err := RunCommand("tar", "czf", outfile, "-C", context.rootdir, ".")

  if err != nil {
    panic(err)
  }
}

type UnpackFilesystemAction struct {
  format string;
  source string;
}

func NewUnpackFilesystemAction(p map[string]interface{}) *UnpackFilesystemAction {
  pf := new(UnpackFilesystemAction)
  pf.source = p["source"].(string)
  pf.format = p["format"].(string)
  return pf
}

func (pf *UnpackFilesystemAction) Run(context YaibContext) {
  infile := path.Join(context.artifactdir, pf.source)

  os.MkdirAll(context.rootdir, 0755)

  fmt.Printf("Unpacking %s\n", infile)
  err := RunCommand("tar", "xzf", infile, "-C", context.rootdir)


  if err != nil {
    panic(err)
  }
}

type OverlayAction struct {
  source string;
}

func NewOverlayAction(p map[string]interface{}) *OverlayAction {
  overlay := new(OverlayAction)
  overlay.source = p["source"].(string)
  return overlay
}

func (overlay *OverlayAction) Run(context YaibContext) {
  sourcedir := path.Join(context.artifactdir, overlay.source)
  RunCommand("find", context.rootdir)

  CopyTree(sourcedir, context.rootdir)
}

type RunAction struct {
  chroot bool;
  script string;
}

func NewRunAction(p map[string]interface{}) *RunAction {
  run := new(RunAction)
  run.chroot = p["chroot"].(bool)
  run.script = p["script"].(string)
  return run
}

func (run *RunAction) Run(context YaibContext) {
  err := RunCommand("systemd-nspawn", "-D", context.rootdir, 
                     "sh", "-c", run.script)
  if err != nil {
    panic(err)
  }
}

type DebootstrapAction struct {
  suite string;
  mirror string;
  script string;
  architecture string;
}

func (d *DebootstrapAction) Run(context YaibContext) {
  TODO: * qemu bind mount 
        * second stage
        * fixup sources.list
  
  err := RunCommand("debootstrap",
                    "--components=target",
                    "--no-check-gpg",
                    "--variant=minbase",
                    "--merged-usr",
                    d.suite,
                    context.rootdir,
                    d.mirror,
                    d.script)
  if err != nil {
    panic(err)
  }
}

func NewDebootstrapAction(p map[string]interface{}) *DebootstrapAction {
  d := new(DebootstrapAction)
  d.suite = p["suite"].(string)
  d.mirror = p["mirror"].(string)
  d.script = p["script"].(string)
  d.architecture = p["architecture"].(string)

  return d
}

func main() {
  var context YaibContext

  context.rootdir = "/tmp/rootdir"

  flag.StringVar(&context.artifactdir, "artifactdir", "", "Artifact directory")
  flag.Parse()

  file := flag.Arg(0)

  if ! fakemachine.InMachine() {
    m := fakemachine.NewMachine()
    var args []string

    if context.artifactdir == "" {
      context.artifactdir, _ = os.Getwd()
    }

    context.artifactdir = CleanPath(context.artifactdir)

    m.AddVolume(context.artifactdir)
    args = append(args, "--artifactdir", context.artifactdir)

    file = CleanPath(file)
    m.AddVolume(path.Dir(file))
    args = append(args, file)

    os.Exit(m.RunInMachineWithArgs(args))
  }

  data, err := ioutil.ReadFile(file)
  if err != nil { panic (err) }

  r := Recipe{}
  err = yaml.Unmarshal(data, &r)
  if err != nil { panic (err) }

  var actions []Action

  for _, v := range r.Actions {
    params := make(map[string]interface{})

    for _,j := range v.Value.(yaml.MapSlice) {
      params[j.Key.(string)] = j.Value
    }

    switch v.Key {
      case "debootstrap":
        actions = append(actions, NewDebootstrapAction(params))
      case "pack_filesystem":
        actions = append(actions, NewPackFilesystemAction(params))
      case "unpack_filesystem":
        actions = append(actions, NewUnpackFilesystemAction(params))
      case "run":
        actions = append(actions, NewRunAction(params))
      case "overlay":
        actions = append(actions, NewOverlayAction(params))
      default:
        panic(fmt.Sprintf("Unknown action: %v", v.Key))
    }
  }

  for _, a := range actions {
    a.Run(context)
  }
}
