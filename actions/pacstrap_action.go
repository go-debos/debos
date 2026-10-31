/*
Pacstrap Action

Construct the target rootfs with pacstrap tool.

	# Yaml syntax:
	- action: pacstrap
	  config: <in-tree pacman.conf file>
	  mirror: <in-tree mirrorlist file>
	  packages: <list of packages to install>

Mandatory properties:

  - config -- the pacman.conf file which will be used through the process
  - mirror -- the mirrorlist file which will be used through the process

Optional properties:

  - packages -- list of packages to install
    If no packages are specified to be installed, pacstrap will install
    the base metapackage by default. The base metapackage is only automatically
    installed when this list is empty, this means that it must be added to the
    list when adding packages.
*/
package actions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

// PacstrapAction implements the pacstrap action.
type PacstrapAction struct {
	debos.BaseAction `yaml:",inline"`

	Config   string   `yaml:"config"`
	Mirror   string   `yaml:"mirror"`
	Packages []string `yaml:"packages"`
}

func (d *PacstrapAction) listOptionFiles(context *debos.Context) ([]string, error) {
	files := []string{}

	if d.Config == "" {
		return nil, fmt.Errorf("no config file set")
	}
	d.Config = debos.CleanPathAt(d.Config, context.RecipeDir)
	files = append(files, d.Config)

	if d.Mirror == "" {
		return nil, fmt.Errorf("no mirror file set")
	}
	d.Mirror = debos.CleanPathAt(d.Mirror, context.RecipeDir)
	files = append(files, d.Mirror)

	return files, nil
}

// Verify checks the properties of the pacstrap action.
func (d *PacstrapAction) Verify(context *debos.Context) error {
	files, err := d.listOptionFiles(context)
	if err != nil {
		return err
	}

	// Check if all needed files exists
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("failed to stat %s: %w", f, err)
		}
	}

	return nil
}

// PreNoMachine prepares the pacstrap action to run without a fake machine.
func (d *PacstrapAction) PreNoMachine(_ *debos.Context) error {
	return fmt.Errorf("action requires fakemachine")
}

// PreMachine prepares the pacstrap action to run in the fake machine.
func (d *PacstrapAction) PreMachine(context *debos.Context, m *fakemachine.Machine, _ *[]string) error {
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

// Run runs the pacstrap action.
func (d *PacstrapAction) Run(context *debos.Context) error {
	files := map[string]string{
		"/etc/pacman.conf":         d.Config,
		"/etc/pacman.d/mirrorlist": d.Mirror,
	}

	// Copy the config/mirrorlist files
	for dest, src := range files {
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path.Dir(dest), err)
		}

		read, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", src, err)
		}

		//nolint:gosec // dest is one of the fixed pacman config paths above.
		if err = os.WriteFile(dest, read, fs.FileMode(0o644)); err != nil {
			return fmt.Errorf("failed to write %s: %w", dest, err)
		}
	}

	// Setup the local keychain, within the fakemachine instance, since we
	// don't have access to the host one.
	// Even if we did, blindly copying it might not be a good idea.
	cmdline := []string{"pacman-key", "--init"}
	if err := (debos.Command{}.Run("pacman-key", cmdline...)); err != nil {
		return fmt.Errorf("couldn't init pacman keyring: %w", err)
	}

	// When there's no explicit keyring suite we populate all available
	cmdline = []string{"pacman-key", "--populate"}
	if err := (debos.Command{}.Run("pacman-key", cmdline...)); err != nil {
		return fmt.Errorf("couldn't populate pacman keyring: %w", err)
	}

	// Run pacstrap
	cmdline = []string{"pacstrap", context.Rootdir}

	if d.Packages != nil {
		cmdline = append(cmdline, d.Packages...)
	}

	err := debos.Command{}.Run("pacstrap", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "var/log/pacman.log")
		catErr := debos.Command{}.Run("pacstrap.log", "cat", log)
		if catErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to show %s: %w", log, catErr))
		}

		return err
	}

	return nil
}
