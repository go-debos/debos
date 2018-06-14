package debos

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
)

type ChrootEnterMethod int

// Return the qemu-user executable name (without a -static suffix)
// to run binaries of the given Debian architecture, or nil if the
// current architecture can run those binaries without using qemu.
func GetQemuUser(debianArch string) (string, error) {
	switch debianArch {
	case "i386":
		if runtime.GOARCH == "386" || runtime.GOARCH == "amd64" {
			return "", nil
		}
		return "qemu-i386", nil
	case "amd64":
		if runtime.GOARCH == "amd64" {
			return "", nil
		}
		return "qemu-x86_64", nil
	case "arm":
		if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" {
			return "", nil
		}
		return "qemu-arm", nil
	case "arm64":
		if runtime.GOARCH == "arm64" {
			return "", nil
		}
		return "qemu-aarch64", nil
	case "powerpc":
		if runtime.GOARCH == "ppc64" {
			return "", nil
		}
		return "qemu-ppc", nil
	case "powerpc64":
		if runtime.GOARCH == "ppc64" {
			return "", nil
		}
		return "qemu-ppc64", nil
	case "ppc64el":
		if runtime.GOARCH == "ppc64le" {
			return "", nil
		}
		return "qemu-ppc64le", nil
	case "mips":
		if runtime.GOARCH == "mips" {
			return "", nil
		}
		return "qemu-mips", nil
	case "mipsel":
		if runtime.GOARCH == "mipsle" {
			return "", nil
		}
		return "qemu-mipsel", nil
	case "mips64":
		if runtime.GOARCH == "mips64" || runtime.GOARCH == "mips" {
			return "", nil
		}
		return "qemu-mips64", nil
	case "mips64el":
		if runtime.GOARCH == "mips64le" || runtime.GOARCH == "mipsle"{
			return "", nil
		}
		return "qemu-mips64el", nil
	case "s390x":
		if runtime.GOARCH == "s390x" {
			return "", nil
		}
		return "qemu-s390x", nil
	default:
		return "", fmt.Errorf("Don't know qemu for Architecture %s", debianArch)
	}
}

const (
	CHROOT_METHOD_NONE   = iota // use nspawn to create the chroot environment
	CHROOT_METHOD_NSPAWN        // No chroot in use
	CHROOT_METHOD_CHROOT        // use chroot to create the chroot environment
)

type Command struct {
	Architecture string            // Architecture of the chroot, nil if same as host
	Dir          string            // Working dir to run command in
	Chroot       string            // Run in the chroot at path
	ChrootMethod ChrootEnterMethod // Method to enter the chroot

	bindMounts []string /// Items to bind mount
	extraEnv   []string // Extra environment variables to set
}

type commandWrapper struct {
	label  string
	buffer *bytes.Buffer
}

func newCommandWrapper(label string) *commandWrapper {
	b := bytes.Buffer{}
	return &commandWrapper{label, &b}
}

func (w commandWrapper) out(atEOF bool) {
	for {
		s, err := w.buffer.ReadString('\n')
		if err == nil {
			log.Printf("%s | %v", w.label, s)
		} else {
			if len(s) > 0 {
				if atEOF && err == io.EOF {
					log.Printf("%s | %v\n", w.label, s)
				} else {
					w.buffer.WriteString(s)
				}
			}
			break
		}
	}
}

func (w commandWrapper) Write(p []byte) (n int, err error) {
	n, err = w.buffer.Write(p)
	w.out(false)
	return
}

func (w *commandWrapper) flush() {
	w.out(true)
}

func NewChrootCommandForContext(context DebosContext) Command {
	c := Command{Architecture: context.Architecture, Chroot: context.Rootdir, ChrootMethod: CHROOT_METHOD_NSPAWN}

	if context.Image != "" {
		path, err := RealPath(context.Image)
		if err == nil {
			c.AddBindMount(path, "")
		} else {
			log.Printf("Failed to get realpath for %s, %v", context.Image, err)
		}
		for _, p := range context.ImagePartitions {
			path, err := RealPath(p.DevicePath)
			if err != nil {
				log.Printf("Failed to get realpath for %s, %v", p.DevicePath, err)
				continue
			}
			c.AddBindMount(path, "")
		}
		c.AddBindMount("/dev/disk", "")
	}

	return c
}

func (cmd *Command) AddEnv(env string) {
	cmd.extraEnv = append(cmd.extraEnv, env)
}

func (cmd *Command) AddEnvKey(key, value string) {
	cmd.extraEnv = append(cmd.extraEnv, fmt.Sprintf("%s=%s", key, value))
}

func (cmd *Command) AddBindMount(source, target string) {
	var mount string
	if target != "" {
		mount = fmt.Sprintf("%s:%s", source, target)
	} else {
		mount = source
	}

	cmd.bindMounts = append(cmd.bindMounts, mount)
}

func (cmd Command) Run(label string, cmdline ...string) error {
	q := newQemuHelper(cmd)
	q.Setup()

	var options []string
	switch cmd.ChrootMethod {
	case CHROOT_METHOD_NONE:
		options = cmdline
	case CHROOT_METHOD_CHROOT:
		options = append(options, "chroot")
		options = append(options, cmd.Chroot)
		options = append(options, cmdline...)
	case CHROOT_METHOD_NSPAWN:
		options = append(options, "systemd-nspawn", "-q", "-D", cmd.Chroot)
		for _, e := range cmd.extraEnv {
			options = append(options, "--setenv", e)

		}
		for _, b := range cmd.bindMounts {
			options = append(options, "--bind", b)

		}
		options = append(options, cmdline...)
	}

	exe := exec.Command(options[0], options[1:]...)
	w := newCommandWrapper(label)

	exe.Stdin = nil
	exe.Stdout = w
	exe.Stderr = w

	if len(cmd.extraEnv) > 0 && cmd.ChrootMethod != CHROOT_METHOD_NSPAWN {
		exe.Env = append(os.Environ(), cmd.extraEnv...)
	}

	// Disable services start/stop for commands running in chroot
	if cmd.ChrootMethod != CHROOT_METHOD_NONE {
		services := ServiceHelper{cmd.Chroot}
		services.Deny()
		defer services.Allow()
	}

	err := exe.Run()
	w.flush()
	q.Cleanup()

	return err
}

type qemuHelper struct {
	qemusrc    string
	qemutarget string
}

func newQemuHelper(c Command) qemuHelper {
	q := qemuHelper{}

	if c.Chroot == "" || c.Architecture == "" {
		return q
	}

	qemu, err := GetQemuUser(c.Architecture)

	if err != nil {
		log.Panicf("%v", err)
	}

	if qemu != "" {
		q.qemusrc = fmt.Sprintf("/usr/bin/%s-static", qemu)
	}

	if q.qemusrc != "" {
		q.qemutarget = path.Join(c.Chroot, q.qemusrc)
	}

	return q
}

func (q qemuHelper) Setup() error {
	if q.qemusrc == "" {
		return nil
	}
	return CopyFile(q.qemusrc, q.qemutarget, 0755)
}

func (q qemuHelper) Cleanup() {
	if q.qemusrc != "" {
		os.Remove(q.qemutarget)
	}
}
