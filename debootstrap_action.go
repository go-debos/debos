package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type DebootstrapAction struct {
	BaseAction     `yaml:",inline"`
	Suite          string
	Mirror         string
	Variant        string
	KeyringPackage string
	Components     []string
}

func (d *DebootstrapAction) RunSecondStage(context DebosContext) error {
	cmdline := []string{
		"/debootstrap/debootstrap",
		"--no-check-gpg",
		"--second-stage"}

	if d.Components != nil {
		s := strings.Join(d.Components, ",")
		cmdline = append(cmdline, fmt.Sprintf("--components=%s", s))
	}

	c := NewChrootCommand(context.rootdir, context.Architecture)
	// Can't use nspawn for debootstrap as it wants to create device nodes
	c.ChrootMethod = CHROOT_METHOD_CHROOT

	return c.Run("Debootstrap (stage 2)", cmdline...)
}

func (d *DebootstrapAction) Run(context *DebosContext) error {
	d.LogStart()
	cmdline := []string{"debootstrap", "--no-check-gpg",
		"--merged-usr"}

	if d.KeyringPackage != "" {
		cmdline = append(cmdline, fmt.Sprintf("--keyring=%s", d.KeyringPackage))
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
	cmdline = append(cmdline, context.rootdir)
	cmdline = append(cmdline, d.Mirror)
	cmdline = append(cmdline, "/usr/share/debootstrap/scripts/unstable")

	err := Command{}.Run("Debootstrap", cmdline...)

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
	srclist, err := os.OpenFile(path.Join(context.rootdir, "etc/apt/sources.list"),
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

	c := NewChrootCommand(context.rootdir, context.Architecture)

	return c.Run("apt clean", "/usr/bin/apt-get", "clean")
}
