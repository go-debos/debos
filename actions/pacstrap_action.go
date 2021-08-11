/*
Pacstrap Action

Construct the target rootfs with pacstrap tool.

Yaml syntax:
 - action: pacstrap
   mirror: <url with placeholders>
   repositories: <list of repositories>

Mandatory properties:

 - mirror -- the full url for the repository, with placeholders for
   $arch and $repo as needed, as would be found in mirrorlist

Optional properties:
 - repositories -- list of repositories to use for packages selection.
   Properties for repositories are described below.

Yaml syntax for repositories:

 repositories:
   - name: repository name
     siglevel: signature checking settings (optional)
*/
package actions

import (
	"fmt"
	"os"
	"path"

	"github.com/go-debos/debos"
)

const configOptionSection = `
[options]
RootDir  = %[1]s
CacheDir = %[1]s/var/cache/pacman/pkg/
GPGDir   = %[1]s/etc/pacman.d/gnupg/
HookDir  = %[1]s/etc/pacman.d/hooks/
HoldPkg  = pacman glibc
Architecture = auto
SigLevel = Required DatabaseOptional TrustAll
`

const configRepoSection = `

[%[1]s]
Server = %[2]s
`

type Repository struct {
	Name     string
	SigLevel string
}

type PacstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Mirror           string
	Repositories     []Repository
}

func (d *PacstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()

	if d.Mirror == "" {
		return fmt.Errorf("No mirror set, aborting.")
	}
	if len(d.Repositories) == 0 {
		return fmt.Errorf("No repositories configured.")
	}

	// Create config for pacstrap
	configPath := path.Join(context.Scratchdir, "pacman.conf")
	f, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Couldn't open pacman config: %v", err)
	}
	_, err = f.WriteString(fmt.Sprintf(configOptionSection, context.Rootdir))
	if err != nil {
		return fmt.Errorf("Couldn't write pacman config: %v", err)
	}
	for _, r := range d.Repositories {
		_, err = f.WriteString(fmt.Sprintf(configRepoSection, r.Name, d.Mirror))
		if err != nil {
			return fmt.Errorf("Couldn't write to pacman config: %v", err)
		}
		if r.SigLevel != "" {
			f.WriteString(fmt.Sprintf("SigLevel = %s\n", r.SigLevel))
		}
	}
	f.Close()

	// Run pacman-key
	cmdline := []string{"pacman-key", "--nocolor", "--config", configPath, "--init"}
	err = debos.Command{}.Run("Pacman-key", cmdline...)
	if err != nil {
		return fmt.Errorf("Couldn't init pacman keyring: %v", err)
	}

	// Run pacstrap
	cmdline = []string{"pacstrap", "-M", "-C", configPath, context.Rootdir}
	err = debos.Command{}.Run("Pacstrap", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "var/log/pacman.log")
		_ = debos.Command{}.Run("pacstrap.log", "cat", log)
		return err
	}

	// Remove pacstrap config
	os.Remove(configPath)

	return nil
}
