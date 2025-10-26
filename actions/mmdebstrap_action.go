/*
mmdebstrap Action

Construct the target rootfs with mmdebstrap tool.

Please keep in mind -- file `/etc/resolv.conf` will be removed after execution.
Most of the OS scripts used by `mmdebstrap` copy `resolv.conf` from the host,
and this may lead to incorrect configuration when becoming part of the created rootfs.

	# Yaml syntax:
	- action: mmdebstrap
	  mirrors: <list of URLs>
	  suite: "name"
	  components: <list of components>
	  variant: "name"
	  keyring-packages:
	  keyring-files:
	  include:
	  dpkg-opts:
	  apt-opts:

Mandatory properties:

- suite -- release code name or symbolic name (e.g. "stable")

Optional properties:

  - mirrors -- list of URLs with Debian-compatible repository
    If no mirror is specified debos will use http://deb.debian.org/debian as default.

- variant -- name of the bootstrap script variant to use

  - components -- list of components to use for packages selection.
    If no components are specified debos will use main as default.

Example:

	components: [ main, contrib ]

- keyring-packages -- list of keyrings for package validation.

- keyring-files -- list keyring files for repository validation.

- merged-usr -- use merged '/usr' filesystem, fallback to distribution default if not set.

- include -- list of packages to install during bootstrap.

- dpkg-opts -- list of arbitrary options to dpkg.

- apt-opts -- list of arbitrary options to apt.
*/
package actions

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

type MmdebstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Suite            string
	Mirrors          []string
	Variant          string
	KeyringPackages  []string `yaml:"keyring-packages"`
	KeyringFiles     []string `yaml:"keyring-files"`
	Components       []string
	MergedUsr        *bool `yaml:"merged-usr"`
	Include          []string
	DpkgOpts         []string `yaml:"dpkg-opts"`
	AptOpts          []string `yaml:"apt-opts"`
}

func NewMmdebstrapAction() *MmdebstrapAction {
	d := MmdebstrapAction{}
	// Use main as default component
	d.Components = []string{"main"}

	return &d
}

func (d *MmdebstrapAction) listOptionFiles(context *debos.Context) []string {
	files := []string{}

	if d.KeyringFiles != nil {
		for i, file := range d.KeyringFiles {
			// normalize in-place so later readers get cleaned paths
			d.KeyringFiles[i] = debos.CleanPathAt(file, context.RecipeDir)
			files = append(files, d.KeyringFiles[i])
		}
	}

	return files
}

func (d *MmdebstrapAction) Verify(context *debos.Context) error {
	if len(d.Suite) == 0 {
		return fmt.Errorf("suite property not specified")
	}

	files := d.listOptionFiles(context)

	// Check if all needed files exists
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (d *MmdebstrapAction) PreMachine(context *debos.Context, m *fakemachine.Machine, _ *[]string) error {
	mounts := d.listOptionFiles(context)

	// Mount configuration files outside of recipes directory
	for _, mount := range mounts {
		m.AddVolume(path.Dir(mount))
	}

	return nil
}

func (d *MmdebstrapAction) Run(context *debos.Context) error {
	cmdline := []string{"mmdebstrap"}

	if d.MergedUsr != nil {
		if *d.MergedUsr {
			cmdline = append(cmdline, "--hook-dir=/usr/share/mmdebstrap/hooks/merged-usr")
		} else {
			cmdline = append(cmdline, "--hook-dir=/usr/share/mmdebstrap/hooks/no-merged-usr")
		}
	}

	if d.KeyringFiles != nil {
		s := strings.Join(d.KeyringFiles, ",")
		cmdline = append(cmdline, fmt.Sprintf("--keyring=%s", s))
	}

	if d.KeyringPackages != nil {
		s := strings.Join(d.KeyringPackages, ",")
		cmdline = append(cmdline, fmt.Sprintf("--include=%s", s))
	}

	if d.Components != nil {
		s := strings.Join(d.Components, ",")
		cmdline = append(cmdline, fmt.Sprintf("--components=%s", s))
	}

	cmdline = append(cmdline, fmt.Sprintf("--architectures=%s", context.Architecture))

	if d.Variant != "" {
		cmdline = append(cmdline, fmt.Sprintf("--variant=%s", d.Variant))
	}

	if d.Include != nil {
		s := strings.Join(d.Include, ",")
		cmdline = append(cmdline, fmt.Sprintf("--include=%s", s))
	}

	if d.DpkgOpts != nil {
		for _, opt := range d.DpkgOpts {
			cmdline = append(cmdline, fmt.Sprintf("--dpkgopt=%s", opt))
		}
	}

	if d.AptOpts != nil {
		for _, opt := range d.AptOpts {
			cmdline = append(cmdline, fmt.Sprintf("--aptopt=%s", opt))
		}
	}

	cmdline = append(cmdline, d.Suite)
	cmdline = append(cmdline, context.Rootdir)

	if d.Mirrors != nil {
		cmdline = append(cmdline, d.Mirrors...)
	}

	/* Make sure files in /etc/apt/ exist inside the fakemachine otherwise
	   mmdebstrap prints a warning about the path not existing. */
	if fakemachine.InMachine() {
		if err := os.MkdirAll(path.Join("/etc/apt/apt.conf.d"), os.ModePerm); err != nil {
			return err
		}
		if err := os.MkdirAll(path.Join("/etc/apt/trusted.gpg.d"), os.ModePerm); err != nil {
			return err
		}
	}

	mmdebstrapErr := debos.Command{}.Run("Mmdebstrap", cmdline...)

	/* Cleanup resolv.conf after mmdebstrap */
	resolvconf := path.Join(context.Rootdir, "/etc/resolv.conf")
	if _, err := os.Stat(resolvconf); !os.IsNotExist(err) {
		if err = os.Remove(resolvconf); err != nil {
			return err
		}
	}

	return mmdebstrapErr
}
