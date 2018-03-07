/*
Debootstrap Action

Construct the target rootfs with debootstrap tool.

Yaml syntax:
 - action: debootstrap
   mirror: URL
   suite: "name"
   components: <list of components>
   variant: "name"
   keyring-package:

Mandatory properties:

- suite -- release code name or symbolic name (e.g. "stable")

Optional properties:

- mirror -- URL with Debian-compatible repository

- variant -- name of the bootstrap script variant to use

- components -- list of components to use for packages selection.
Example:
 components: [ main, contrib ]

- keyring-package -- keyring for packages validation. Currently ignored.

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
	Components       []string
	MergedUsr        bool `yaml:"merged-usr"`
}

func NewDebootstrapAction() *DebootstrapAction {
	d := DebootstrapAction{}
	// Use filesystem with merged '/usr' by default
	d.MergedUsr = true
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

	return c.Run("Debootstrap (stage 2)", cmdline...)
}

func (d *DebootstrapAction) Run(context *debos.DebosContext) error {
	d.LogStart()
	cmdline := []string{"debootstrap", "--no-check-gpg"}

	if d.MergedUsr {
		cmdline = append(cmdline, "--merged-usr")
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

	c := debos.NewChrootCommandForContext(*context)

	return c.Run("apt clean", "/usr/bin/apt-get", "clean")
}
