/*
Pacstrap Action

Construct the target rootfs with pacstrap tool.

	# Yaml syntax:
	- action: pacstrap
	  config: <in-tree pacman.conf file>
	  mirror: <in-tree mirrorlist file>

Mandatory properties:

  - config -- the pacman.conf file which will be used through the process
  - mirror -- the mirrorlist file which will be used through the process
*/
package actions

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

type PacstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Config           string `yaml:"config"`
	Mirror           string `yaml:"mirror"`
}

func (d *PacstrapAction) listOptionFiles(context *debos.DebosContext) ([]string, error) {
	files := []string{}

	if d.Config == "" {
		return nil, fmt.Errorf("No config file set")
	}
	d.Config = debos.CleanPathAt(d.Config, context.RecipeDir)
	files = append(files, d.Config)

	if d.Mirror == "" {
		return nil, fmt.Errorf("No mirror file set")
	}
	d.Mirror = debos.CleanPathAt(d.Mirror, context.RecipeDir)
	files = append(files, d.Mirror)

	return files, nil
}

func (d *PacstrapAction) Verify(context *debos.DebosContext) error {
	files, err := d.listOptionFiles(context)
	if err != nil {
		return err
	}

	// Check if all needed files exists
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (d *PacstrapAction) PreNoMachine(context *debos.DebosContext) error {
	return fmt.Errorf("action requires fakemachine")
}

func (d *PacstrapAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine, args *[]string) error {
	mounts, err := d.listOptionFiles(context)
	if err != nil {
		return err
	}

	// Mount configuration files outside of recipes directory
	for _, mount := range mounts {
		m.AddVolume(path.Dir(mount))
	}

	return nil
}

func (d *PacstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()

	files := map[string]string{
		"/etc/pacman.conf":         d.Config,
		"/etc/pacman.d/mirrorlist": d.Mirror,
	}

	// Copy the config/mirrorlist files
	for dest, src := range files {
		if err := os.MkdirAll(path.Dir(dest), 0755); err != nil {
			return err
		}

		read, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}

		if err = ioutil.WriteFile(dest, read, 0644); err != nil {
			return err
		}
	}

	// Setup the local keychain, within the fakemachine instance, since we
	// don't have access to the host one.
	// Even if we did, blindly copying it might not be a good idea.
	cmdline := []string{"pacman-key", "--init"}
	if err := (debos.Command{}.Run("pacman-key", cmdline...)); err != nil {
		return fmt.Errorf("Couldn't init pacman keyring: %v", err)
	}

	// When there's no explicit keyring suite we populate all available
	cmdline = []string{"pacman-key", "--populate"}
	if err := (debos.Command{}.Run("pacman-key", cmdline...)); err != nil {
		return fmt.Errorf("Couldn't populate pacman keyring: %v", err)
	}

	// Run pacstrap
	cmdline = []string{"pacstrap", context.Rootdir}
	if err := (debos.Command{}.Run("pacstrap", cmdline...)); err != nil {
		log := path.Join(context.Rootdir, "var/log/pacman.log")
		_ = debos.Command{}.Run("pacstrap.log", "cat", log)
		return err
	}

	return nil
}
