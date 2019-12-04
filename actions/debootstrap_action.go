/*
Debootstrap Action

Construct the target rootfs with debootstrap tool.

Please keep in mind -- file `/etc/resolv.conf` will be removed after execution.
Most of the OS scripts used by `debootstrap` copy `resolv.conf` from the host,
and this may lead to incorrect configuration when becoming part of the created rootfs.

Yaml syntax:
 - action: debootstrap
   mirror: URL
   suite: "name"
   components: <list of components>
   variant: "name"
   keyring-package:
   keyring-file:

Mandatory properties:

- suite -- release code name or symbolic name (e.g. "stable")

Optional properties:

- check-gpg -- verify GPG signatures on Release files, true by default

- mirror -- URL with Debian-compatible repository
 If no mirror is specified debos will use http://deb.debian.org/debian as default.

- variant -- name of the bootstrap script variant to use

- components -- list of components to use for packages selection.
 If no components are specified debos will use main as default.

Example:
 components: [ main, contrib ]

- keyring-package -- keyring for package validation.

- keyring-file -- keyring file for repository validation.

- merged-usr -- use merged '/usr' filesystem, true by default.
*/
package actions

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

type DebootstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Suite            string
	Mirror           string
	Variant          string
	KeyringPackage   string `yaml:"keyring-package"`
	KeyringFile      string `yaml:"keyring-file"`
	Components       []string
	MergedUsr        bool `yaml:"merged-usr"`
	CheckGpg         bool `yaml:"check-gpg"`
}

func NewDebootstrapAction() *DebootstrapAction {
	d := DebootstrapAction{}
	// Use filesystem with merged '/usr' by default
	d.MergedUsr = true
	// Be secure by default
	d.CheckGpg = true
	// Use main as default component
	d.Components = []string {"main"}
	// Set generic default mirror
	d.Mirror = "http://deb.debian.org/debian"

	return &d
}

func (d *DebootstrapAction) RunSecondStage(context debos.DebosContext) error {
	cmdline := []string{
		"/debootstrap/debootstrap",
		"--no-check-gpg",
		"--second-stage"}

	if d.Components != nil {
		s := strings.Join(d.Components, ",")
		cmdline = append(cmdline, fmt.Sprintf("--components=%s", s))
	}

	c := debos.NewChrootCommandForContext(context)
	// Can't use nspawn for debootstrap as it wants to create device nodes
	c.ChrootMethod = debos.CHROOT_METHOD_CHROOT

	err := c.Run("Debootstrap (stage 2)", cmdline...)

	if (err != nil) {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		_ = debos.Command{}.Run("debootstrap.log", "cat", log)
	}

	return err
}

func (d *DebootstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()
	cmdline := []string{"debootstrap"}

	if d.MergedUsr {
		cmdline = append(cmdline, "--merged-usr")
	} else {
		cmdline = append(cmdline, "--no-merged-usr")
	}

	if !d.CheckGpg {
		cmdline = append(cmdline, fmt.Sprintf("--no-check-gpg"))
	} else if d.KeyringFile != "" {
		path := debos.CleanPathAt(d.KeyringFile, context.RecipeDir)
		cmdline = append(cmdline, fmt.Sprintf("--keyring=%s", path))
	}

	if d.KeyringPackage != "" {
		cmdline = append(cmdline, fmt.Sprintf("--include=%s", d.KeyringPackage))
	}

	if d.Components != nil {
		s := strings.Join(d.Components, ",")
		cmdline = append(cmdline, fmt.Sprintf("--components=%s", s))
	}

	/* FIXME drop the hardcoded amd64 assumption" */
	foreign := context.Architecture != "amd64"

	if foreign {
		cmdline = append(cmdline, "--foreign")
		cmdline = append(cmdline, fmt.Sprintf("--arch=%s", context.Architecture))

	}

	if d.Variant != "" {
		cmdline = append(cmdline, fmt.Sprintf("--variant=%s", d.Variant))
	}

	cmdline = append(cmdline, d.Suite)
	cmdline = append(cmdline, context.Rootdir)
	cmdline = append(cmdline, d.Mirror)
	cmdline = append(cmdline, "/usr/share/debootstrap/scripts/unstable")

	err := debos.Command{}.Run("Debootstrap", cmdline...)

	if err != nil {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		_ = debos.Command{}.Run("debootstrap.log", "cat", log)
		return err
	}

	if foreign {
		err = d.RunSecondStage(*context)
		if err != nil {
			return err
		}
	}

	/* HACK */
	srclist, err := os.OpenFile(path.Join(context.Rootdir, "etc/apt/sources.list"),
		os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	_, err = io.WriteString(srclist, fmt.Sprintf("deb %s %s %s\n",
		d.Mirror,
		d.Suite,
		strings.Join(d.Components, " ")))
	if err != nil {
		return err
	}
	srclist.Close()

	/* Cleanup resolv.conf after debootstrap */
	resolvconf := path.Join(context.Rootdir, "/etc/resolv.conf")
	if _, err = os.Stat(resolvconf); !os.IsNotExist(err) {
		if err = os.Remove(resolvconf); err != nil {
			return err
		}
	}

	c := debos.NewChrootCommandForContext(*context)

	return c.Run("apt clean", "/usr/bin/apt-get", "clean")
}
