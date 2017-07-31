package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type DebootstrapAction struct {
	*BaseAction
	Suite          string
	Mirror         string
	Variant        string
	KeyringPackage string
	Components     []string
}

func (d *DebootstrapAction) RunSecondStage(context YaibContext) {
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

	err := c.Run("Debootstrap (stage 2)", cmdline...)

	if err != nil {
		log.Panic(err)
	}

}

func (d *DebootstrapAction) Run(context *YaibContext) {
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
		panic(err)
	}

	if foreign {
		d.RunSecondStage(*context)
	}

	/* HACK */
	srclist, err := os.OpenFile(path.Join(context.rootdir, "etc/apt/sources.list"),
		os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(srclist, fmt.Sprintf("deb %s %s %s\n",
		d.Mirror,
		d.Suite,
		strings.Join(d.Components, " ")))
	if err != nil {
		panic(err)
	}
	srclist.Close()

	c := NewChrootCommand(context.rootdir, context.Architecture)

	err = c.Run("apt clean", "/usr/bin/apt-get", "clean")
	if err != nil {
		panic(err)
	}
}
