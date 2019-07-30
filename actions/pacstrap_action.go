/*
Pacstrap Action

Construct the target rootfs with pacstrap tool.

Yaml syntax:
 - action: pacstrap
   mirror: URL

Optional properties:

- mirror -- URL with ArchLinux-compatible repository
 If no mirror is specified debos will use http://mirrors.kernel.org/archlinux as default.
*/
package actions

import (
	"fmt"
	"os"
	"path"

	"github.com/go-debos/debos"
)

const pacmanConfig = `
[options]
RootDir  = %[1]s
CacheDir = %[1]s/var/cache/pacman/pkg/
GPGDir   = %[1]s/etc/pacman.d/gnupg/
HookDir  = %[1]s/etc/pacman.d/hooks/
HoldPkg  = pacman glibc
Architecture = auto
SigLevel = Required DatabaseOptional TrustAll

[core]
Server = %[2]s/$repo/os/$arch

[extra]
Server = %[2]s/$repo/os/$arch

[community]
Server = %[2]s/$repo/os/$arch
`

type PacstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Mirror string
}

func NewPacstrapAction() *PacstrapAction {
	d := PacstrapAction{}
	// Set generic default mirror
	d.Mirror = "http://mirrors.kernel.org/archlinux"

	return &d
}

func (d *PacstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()

	// Create config for pacstrap
	configPath := path.Join(context.Scratchdir, "pacman.conf")
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open pacman config: %v", err)
	}
	_, err = f.WriteString(fmt.Sprintf(pacmanConfig, context.Rootdir, d.Mirror))
	if err != nil {
		return fmt.Errorf("Couldn't write pacman config: %v", err)
	}
	f.Close()

	// Create base layout for pacman-key
	err = os.MkdirAll(path.Join(context.Rootdir, "var", "lib", "pacman"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create var/lib/pacman in image: %v", err)
	}
	err = os.MkdirAll(path.Join(context.Rootdir, "etc", "pacman.d", "gnupg"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc/pacman.d/gnupg in image: %v", err)
	}

	// Run pacman-key
	cmdline := []string{"pacman-key", "--nocolor", "--config", configPath, "--init"}
	err = debos.Command{}.Run("Pacman-key", cmdline...)
	if err != nil {
		return fmt.Errorf("Couldn't init pacman keyring: %v", err)
	}

	cmdline = []string{"pacman-key", "--nocolor", "--config", configPath, "--populate", "archlinux"}
	err = debos.Command{}.Run("Pacman-key", cmdline...)
	if err != nil {
		return fmt.Errorf("Couldn't populate pacman keyring: %v", err)
	}

	// Run pacstrap
	cmdline = []string{"pacstrap", "-GM", "-C", configPath, context.Rootdir}
	err = debos.Command{}.Run("Pacstrap", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "var/log/pacman.log")
		_ = debos.Command{}.Run("pacstrap.log", "cat", log)
		return err
	}

	// Remove pacstrap config
	os.Remove(configPath)

	// Configure mirror
	mirrorlistPath := path.Join(context.Rootdir, "etc", "pacman.d", "mirrorlist")
	err = os.Rename(mirrorlistPath, mirrorlistPath + ".bck")
	if err != nil {
		return fmt.Errorf("Couldn't move pacman mirrorlist in image: %v", err)
	}

	f, err = os.OpenFile(mirrorlistPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open pacman mirrorlist in image: %v", err)
	}
	_, err = f.WriteString(fmt.Sprintf("Server = %s/$repo/os/$arch\n", d.Mirror))
	if err != nil {
		return fmt.Errorf("Couldn't write pacman mirrorlist in image: %v", err)
	}
	f.Close()

	return nil
}
