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

  - debootstrap-suite -- name of the suite to use for debootstrap itself. This usually
    selects the builtin debootstrap script and is typically the direct parent of a
    downstream (e.g. `buster` for older Apertis releases, `jammy` for Ubuntu derivatives).

  - debootstrap-script -- path (inside the recipe origin) to a custom debootstrap
    script to use instead of the builtin ones.

If neither `debootstrap-suite“ nor `debootstrap-script` is set, debos will use the
builtin script for the `suite` if one exists, otherwise it falls back to the
`unstable` debootstrap script.
*/
package actions

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

type DebootstrapAction struct {
	debos.BaseAction  `yaml:",inline"`
	DebootstrapSuite  string `yaml:"debootstrap-suite"`
	Suite             string
	Mirror            string
	Variant           string
	KeyringPackage    string `yaml:"keyring-package"`
	KeyringFile       string `yaml:"keyring-file"`
	Certificate       string
	PrivateKey        string `yaml:"private-key"`
	Components        []string
	MergedUsr         bool   `yaml:"merged-usr"`
	CheckGpg          bool   `yaml:"check-gpg"`
	DebootstrapScript string `yaml:"debootstrap-script"`
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

func (d *DebootstrapAction) listOptionFiles(context *debos.Context) []string {
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

	if d.DebootstrapScript != "" {
		d.DebootstrapScript = debos.CleanPathAt(d.DebootstrapScript, context.RecipeDir)
		files = append(files, d.DebootstrapScript)
	}

	return files
}

func (d *DebootstrapAction) Verify(context *debos.Context) error {
	if len(d.Suite) == 0 {
		return fmt.Errorf("suite property not specified")
	}

	if d.DebootstrapSuite != "" && d.DebootstrapScript != "" {
		return fmt.Errorf("only one of debootstrap-suite or debootstrap-script may be set")
	}

	files := d.listOptionFiles(context)

	// Check if all needed files exist
	for _, f := range files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return err
		}
	}

	if d.DebootstrapSuite != "" {
		suiteScript := getDebootstrapScriptPath(d.Suite)
		if _, err := os.Stat(suiteScript); err == nil {
			return fmt.Errorf("suite %q already has a builtin debootstrap script; debootstrap-suite should not be set", d.Suite)
		}

		parentScript := getDebootstrapScriptPath(d.DebootstrapSuite)
		if _, err := os.Stat(parentScript); err != nil {
			return fmt.Errorf("cannot find debootstrap script for debootstrap-suite %q", d.DebootstrapSuite)
		}
	}

	return nil
}

func (d *DebootstrapAction) PreMachine(context *debos.Context, m *fakemachine.Machine, _ *[]string) error {
	mounts := d.listOptionFiles(context)

	// Mount configuration files outside of recipes directory
	for _, mount := range mounts {
		m.AddVolume(path.Dir(mount))
	}

	return nil
}

func (d *DebootstrapAction) RunSecondStage(context debos.Context) error {
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
	c.ChrootMethod = debos.ChrootMethodChroot

	err := c.Run("Debootstrap (stage 2)", cmdline...)

	if err != nil {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		_ = debos.Command{}.Run("debootstrap.log", "cat", log)
	}

	return err
}

// shouldExcludeUsrIsMerged returns true for Debian suites where usr-is-merged
// is not available.
func shouldExcludeUsrIsMerged(suite string) bool {
	switch strings.ToLower(suite) {
	case "etch",
		"lenny",
		"squeeze",
		"wheezy",
		"jessie",
		"stretch",
		"buster",
		"bullseye":
		return true
	default:
		// Default to "no workaround" for anything unknown/new (Debian >= bookworm
		// and derivatives)
		return false
	}
}

func getDebootstrapScriptPath(script string) string {
	return path.Join("/usr/share/debootstrap/scripts/", script)
}

func (d *DebootstrapAction) Run(context *debos.Context) error {
	cmdline := []string{"debootstrap"}

	if d.MergedUsr {
		cmdline = append(cmdline, "--merged-usr")
	} else {
		cmdline = append(cmdline, "--no-merged-usr")
	}

	if !d.CheckGpg {
		cmdline = append(cmdline, "--no-check-gpg")
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

	/* Only works for amd64, arm64 and riscv64 hosts, which should be enough */
	foreign := context.Architecture != runtime.GOARCH

	if foreign {
		cmdline = append(cmdline, "--foreign")
		cmdline = append(cmdline, fmt.Sprintf("--arch=%s", context.Architecture))
	}

	if d.Variant != "" {
		cmdline = append(cmdline, fmt.Sprintf("--variant=%s", d.Variant))
	}

	// Determine which debootstrap script to use and which suite we apply the
	// usr-is-merged workaround for.
	var scriptPath string
	var suiteForWorkaround string

	if d.DebootstrapScript != "" {
		// Custom script specified by the user
		scriptPath = d.DebootstrapScript

		// When using a custom script we assume any distro-specific workarounds
		// are handled there and do not apply the usr-is-merged workaround here.
		suiteForWorkaround = ""
	} else if d.DebootstrapSuite != "" {
		// Explicit parent/debootstrap suite
		suiteForWorkaround = d.DebootstrapSuite
		scriptPath = getDebootstrapScriptPath(suiteForWorkaround)
	} else {
		// Automatic behaviour: use suite's builtin script if available, otherwise
		// fall back to unstable (current behaviour) and suggest debootstrap-suite.
		suiteForWorkaround = d.Suite
		scriptPath = getDebootstrapScriptPath(suiteForWorkaround)
		if _, err := os.Stat(scriptPath); err != nil {
			log.Printf("cannot find debootstrap script %s, falling back to unstable\n", scriptPath)
			suiteForWorkaround = "unstable"
			scriptPath = getDebootstrapScriptPath(suiteForWorkaround)
			if _, err := os.Stat(scriptPath); err != nil {
				return errors.New("cannot find debootstrap script for unstable")
			}
			log.Printf("using fallback debootstrap script %s; consider setting debootstrap-suite", scriptPath)
		}
	}

	// workaround for https://github.com/go-debos/debos/issues/361
	// Apply the workaround only for known-old suites
	if suiteForWorkaround != "" && shouldExcludeUsrIsMerged(suiteForWorkaround) {
		log.Printf("excluding usr-is-merged as package is not in suite %s\n", suiteForWorkaround)
		cmdline = append(cmdline, "--exclude=usr-is-merged")
	}

	cmdline = append(cmdline, d.Suite)
	cmdline = append(cmdline, context.Rootdir)
	cmdline = append(cmdline, d.Mirror)
	cmdline = append(cmdline, scriptPath)

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
