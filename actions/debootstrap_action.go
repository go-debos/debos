/*
Debootstrap Action

Construct the target rootfs with debootstrap tool.

Please keep in mind -- file `/etc/resolv.conf` will be removed after execution.
Most of the OS scripts used by `debootstrap` copy `resolv.conf` from the host,
and this may lead to incorrect configuration when becoming part of the created rootfs.

 # Yaml syntax:
 - action: debootstrap
   mirror: URL
   suite: "name"
   components: <list of components>
   variant: "name"
   keyring-package:
   keyring-file:
   certificate:
   private-key:

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

- certificate -- client certificate stored in file to be used for downloading packages from the server.

- private-key -- provide the client's private key in a file separate from the certificate.
*/
package actions

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

type DebootstrapAction struct {
	debos.BaseAction `yaml:",inline"`
	Suite            string
	Mirror           string
	Variant          string
	KeyringPackage   string `yaml:"keyring-package"`
	KeyringFile      string `yaml:"keyring-file"`
	Certificate      string
	PrivateKey       string `yaml:"private-key"`
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
	d.Components = []string{"main"}
	// Set generic default mirror
	d.Mirror = "http://deb.debian.org/debian"

	return &d
}

func (d *DebootstrapAction) listOptionFiles(context *debos.DebosContext) []string {
	files := []string{}
	if d.Certificate != "" {
		d.Certificate = debos.CleanPathAt(d.Certificate, context.RecipeDir)
		files = append(files, d.Certificate)
	}

	if d.PrivateKey != "" {
		d.PrivateKey = debos.CleanPathAt(d.PrivateKey, context.RecipeDir)
		files = append(files, d.PrivateKey)
	}

	if d.KeyringFile != "" {
		d.KeyringFile = debos.CleanPathAt(d.KeyringFile, context.RecipeDir)
		files = append(files, d.KeyringFile)
	}

	return files
}

func (d *DebootstrapAction) Verify(context *debos.DebosContext) error {
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

func (d *DebootstrapAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine, args *[]string) error {

	mounts := d.listOptionFiles(context)

	// Mount configuration files outside of recipes directory
	for _, mount := range mounts {
		m.AddVolume(path.Dir(mount))
	}

	return nil
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

	if err != nil {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		_ = debos.Command{}.Run("debootstrap.log", "cat", log)
	}

	return err
}

// Guess if suite is something before usr-is-merged was introduced
func (d *DebootstrapAction) isLikelyOldSuite() bool {
	switch strings.ToLower(d.Suite) {
	case "sid", "unstable":
		return false
	case "testing":
		return false
	case "bookworm":
		return false
	case "trixie":
		return false
	case "forky":
		return false
	default:
		return true
	}
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
		cmdline = append(cmdline, fmt.Sprintf("--keyring=%s", d.KeyringFile))
	}

	if d.KeyringPackage != "" {
		cmdline = append(cmdline, fmt.Sprintf("--include=%s", d.KeyringPackage))
	}

	if d.Certificate != "" {
		cmdline = append(cmdline, fmt.Sprintf("--certificate=%s", d.Certificate))
	}

	if d.PrivateKey != "" {
		cmdline = append(cmdline, fmt.Sprintf("--private-key=%s", d.PrivateKey))
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

	// workaround for https://github.com/go-debos/debos/issues/361
	if d.isLikelyOldSuite() {
		log.Println("excluding usr-is-merged as package is not in suite")
		cmdline = append(cmdline, "--exclude=usr-is-merged")
	}

	cmdline = append(cmdline, d.Suite)
	cmdline = append(cmdline, context.Rootdir)
	cmdline = append(cmdline, d.Mirror)
	cmdline = append(cmdline, "/usr/share/debootstrap/scripts/unstable")

	/* Make sure /etc/apt/apt.conf.d exists inside the fakemachine otherwise
	   debootstrap prints a warning about the path not existing. */
	if fakemachine.InMachine() {
		if err := os.MkdirAll(path.Join("/etc/apt/apt.conf.d"), os.ModePerm); err != nil {
			return err
		}
	}

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
