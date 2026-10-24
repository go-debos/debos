package debos

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-debos/fakemachine"
)

// ChrootEnterMethod selects how a Command enters its chroot.
type ChrootEnterMethod int

// Available ChrootEnterMethod values.
const (
	ChrootMethodNone    ChrootEnterMethod = iota // No chroot in use
	ChrootMethodNspawn                           // use nspawn to create the chroot environment
	ChrootMethodChroot                           // use chroot to create the chroot environment
	ChrootMethodUnshare                          // use chroot in a private mount namespace with the API filesystems set up
)

// unshareChrootScript sets up the API filesystems and bind mounts in a private
// mount namespace and then enters the chroot; used where systemd-nspawn can't
// run, i.e. in the fakemachine unshare and incus backends which are themselves
// inside a user namespace.
// Arguments: root, then source/target pairs for bind mounts, then "--" and the
// command line.
const unshareChrootScript = `set -e
root="$1"; shift
mkdir -p "$root/proc" "$root/sys" "$root/dev" "$root/run"
mount --rbind /proc "$root/proc"
mount --rbind /sys "$root/sys"
mount --rbind /dev "$root/dev"
mount -t tmpfs -o mode=755 run "$root/run"
while [ "$1" != "--" ]; do
	if [ -d "$1" ]; then
		mkdir -p "$root$2"
	else
		mkdir -p "$(dirname "$root$2")"
		touch "$root$2"
	fi
	mount --rbind "$1" "$root$2"
	shift 2
done
shift
exec chroot "$root" "$@"
`

// Command describes how to run a program on the host or inside a chroot.
// The zero value runs the program directly on the host.
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

func (w *commandWrapper) out(atEOF bool) {
	for {
		s, err := w.buffer.ReadString('\n')
		if err == nil {
			log.Printf("%s | %v", w.label, s)

			continue
		}

		switch {
		case len(s) == 0:
		case atEOF && err == io.EOF:
			log.Printf("%s | %v\n", w.label, s)
		default:
			w.buffer.WriteString(s)
		}

		return
	}
}

func (w *commandWrapper) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if err != nil {
		return n, fmt.Errorf("failed to write to buffer: %w", err)
	}
	w.out(false)

	return n, nil
}

func (w *commandWrapper) flush() {
	w.out(true)
}

// NewChrootCommandForContext returns a Command that runs inside the root
// directory of the context using systemd-nspawn, with the environment
// variables of the context.
func NewChrootCommandForContext(context Context) Command {
	c := Command{Architecture: context.Architecture, Chroot: context.Rootdir, ChrootMethod: ChrootMethodNspawn}

	// systemd-nspawn needs real root, which the unshare backend doesn't have;
	// in the incus backend it can't mount proc after switching root, as the
	// kernel only allows that in a user namespace if a proc is still visible
	switch fakemachine.InMachineBackend() {
	case "unshare", "incus":
		c.ChrootMethod = ChrootMethodUnshare
	}

	if context.EnvironVars != nil {
		for k, v := range context.EnvironVars {
			c.AddEnv(fmt.Sprintf("%s=%s", k, v))
		}
	}

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

// AddEnv adds an environment variable, in KEY=value form, to the command.
func (cmd *Command) AddEnv(env string) {
	cmd.extraEnv = append(cmd.extraEnv, env)
}

// AddEnvKey adds the environment variable key with the given value.
func (cmd *Command) AddEnvKey(key, value string) {
	cmd.extraEnv = append(cmd.extraEnv, fmt.Sprintf("%s=%s", key, value))
}

// AddBindMount bind mounts source at target inside the chroot.
func (cmd *Command) AddBindMount(source, target string) {
	var mount string
	if target != "" {
		mount = fmt.Sprintf("%s:%s", source, target)
	} else {
		mount = source
	}

	cmd.bindMounts = append(cmd.bindMounts, mount)
}

func (cmd *Command) saveResolvConf() ([sha256.Size]byte, error) {
	hostconf := "/etc/resolv.conf"
	chrootedconf := path.Join(cmd.Chroot, hostconf)
	savedconf := chrootedconf + ".debos"
	var sum [sha256.Size]byte

	if cmd.ChrootMethod == ChrootMethodNone {
		return sum, nil
	}

	// There may not be an existing resolv.conf
	if err := os.Rename(chrootedconf, savedconf); err != nil && !errors.Is(err, os.ErrNotExist) {
		return sum, fmt.Errorf("failed to rename %s to %s: %w", chrootedconf, savedconf, err)
	}

	/* Expect a relatively small file here */
	data, err := os.ReadFile(hostconf)
	if err != nil {
		return sum, fmt.Errorf("failed to read %s: %w", hostconf, err)
	}
	out := []byte("# Automatically generated by Debos\n")
	out = append(out, data...)

	sum = sha256.Sum256(out)

	//nolint:gosec // chrootedconf is the fixed resolv.conf path inside the chroot.
	err = os.WriteFile(chrootedconf, out, 0o644)
	if err != nil {
		return sum, fmt.Errorf("failed to write %s: %w", chrootedconf, err)
	}

	return sum, nil
}

func (cmd *Command) restoreResolvConf(sum [sha256.Size]byte) (err error) {
	hostconf := "/etc/resolv.conf"
	chrootedconf := path.Join(cmd.Chroot, hostconf)
	savedconf := chrootedconf + ".debos"

	if cmd.ChrootMethod == ChrootMethodNone {
		return nil
	}

	// Remove the original copy anyway
	defer func() {
		if removeErr := os.Remove(savedconf); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("failed to remove %s: %w", savedconf, removeErr))
		}
	}()

	fi, err := os.Lstat(chrootedconf)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// resolv.conf was removed during the command call
			// Nothing to do with it -- file has been changed anyway
			return nil
		}

		return fmt.Errorf("failed to stat %s: %w", chrootedconf, err)
	}

	mode := fi.Mode()
	switch {
	case mode.IsRegular():
		// Try to calculate checksum
		data, err := os.ReadFile(chrootedconf)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", chrootedconf, err)
		}
		currentsum := sha256.Sum256(data)

		// Leave the changed resolv.conf untouched
		if bytes.Equal(currentsum[:], sum[:]) {
			// Remove the generated version
			if err := os.Remove(chrootedconf); err != nil {
				return fmt.Errorf("failed to remove %s: %w", chrootedconf, err)
			}

			// Restore the original version
			if err := os.Rename(savedconf, chrootedconf); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to rename %s to %s: %w", savedconf, chrootedconf, err)
			}
		}
	case mode&os.ModeSymlink != 0:
		// If the 'resolv.conf' is a symlink
		// Nothing to do with it -- file has been changed anyway
	default:
		// File is not regular or symlink
		// Let's get out here with verbose message
		log.Printf("Warning: /etc/resolv.conf inside the chroot is not a regular file")
	}

	return nil
}

// Run runs cmdline, in the chroot if one is configured, logging its output
// with the label as a prefix.
func (cmd Command) Run(label string, cmdline ...string) (err error) {
	if len(cmdline) == 0 {
		return errors.New("no command given")
	}

	var options []string
	switch cmd.ChrootMethod {
	case ChrootMethodNone:
		options = cmdline
	case ChrootMethodChroot:
		options = append(options, "chroot")
		options = append(options, cmd.Chroot)
		options = append(options, cmdline...)
	case ChrootMethodNspawn:
		// We use own resolv.conf handling
		options = append(options, "systemd-nspawn", "-q")
		options = append(options, "--resolv-conf=off")
		options = append(options, "--timezone=off")
		options = append(options, "--register=no")
		//nolint:gosec // The machine name only needs to be unique, not unpredictable.
		options = append(options, fmt.Sprintf("--machine=debos-%d", rand.Int63()))
		options = append(options, "--keep-unit")
		options = append(options, "--console=pipe")
		for _, e := range cmd.extraEnv {
			options = append(options, "--setenv", e)
		}
		for _, b := range cmd.bindMounts {
			options = append(options, "--bind", b)
		}
		options = append(options, "-D", cmd.Chroot)
		options = append(options, cmdline...)
	case ChrootMethodUnshare:
		options = append(options, "unshare", "--mount", "--propagation", "private",
			"/bin/sh", "-c", unshareChrootScript, "sh", cmd.Chroot)
		for _, b := range cmd.bindMounts {
			source, target, found := strings.Cut(b, ":")
			if !found {
				target = source
			}
			options = append(options, source, target)
		}
		options = append(options, "--")
		options = append(options, cmdline...)
	default:
		return fmt.Errorf("unknown chroot method %d", cmd.ChrootMethod)
	}

	//nolint:gosec // Running recipe-provided commands is debos' purpose.
	exe := exec.Command(options[0], options[1:]...)
	w := newCommandWrapper(label)

	exe.Stdin = nil
	exe.Stdout = w
	exe.Stderr = w

	defer w.flush()

	if len(cmd.extraEnv) > 0 && cmd.ChrootMethod != ChrootMethodNspawn {
		exe.Env = append(os.Environ(), cmd.extraEnv...)
	}

	// Disable services start/stop for commands running in chroot
	if cmd.ChrootMethod != ChrootMethodNone {
		services := ServiceHelper{cmd.Chroot}
		if err := services.Deny(); err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, services.Allow())
		}()
	}

	// Save the original resolv.conf and copy version from host
	resolvsum, err := cmd.saveResolvConf()
	if err != nil {
		return err
	}

	if err = exe.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", cmdline[0], err)
	}

	// Restore the original resolv.conf if not changed
	if err = cmd.restoreResolvConf(resolvsum); err != nil {
		return err
	}

	return nil
}
