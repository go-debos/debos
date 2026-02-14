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

For bootstrapping Debian derivatives, the following optional properties may be
helpful to choose the right debootstrap script. Only one of these properties may
be set.

  - debootstrap-suite -- name of the suite whose debootstrap script should be
    used. When bootstrapping derivatives, this should be set to the suite the
    derivative is based on (e.g. `buster` for older Apertis releases, `jammy` for
    Ubuntu derivatives). If unset, it is assumed to be the same value as `suite`.
    A debootstrap script must exist for this suite, and it is an error to set this
    property if `suite` already has its own debootstrap script.

  - debootstrap-script -- path (inside the recipe origin) to a custom debootstrap
    script to use.

If neither `debootstrap-suite` nor `debootstrap-script` is set, debos will attempt
to use a debootstrap script with the `suite` name if it exists, otherwise falls
back to the `unstable` debootstrap script.

The suite used to select the debootstrap script also determines whether the
`usr-is-merged` package is excluded, which is only done for Debian suites up to
and including `bullseye` as the package is not available there. Derivatives
based on those suites should set `debootstrap-suite` accordingly.
*/
package actions

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

// DebootstrapAction implements the debootstrap action.
type DebootstrapAction struct {
	debos.BaseAction `yaml:",inline"`

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
	DebootstrapSuite  string `yaml:"debootstrap-suite"`
	DebootstrapScript string `yaml:"debootstrap-script"`

	// Absolute path to the debootstrap script.
	scriptPath string

	// Suite used to select the debootstrap script and workarounds.
	scriptSuite string
}

// NewDebootstrapAction returns a new DebootstrapAction.
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

// Verify checks the properties of the debootstrap action.
func (d *DebootstrapAction) Verify(context *debos.Context) error {
	if len(d.Suite) == 0 {
		return errors.New("suite property not specified")
	}

	if len(d.DebootstrapSuite) > 0 && len(d.DebootstrapScript) > 0 {
		return errors.New("can't set both 'debootstrap-suite' and 'debootstrap-script' properties")
	}

	files := d.listOptionFiles(context)

	// Check if all needed files exist
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return fmt.Errorf("failed to stat %s: %w", f, err)
		}
	}

	// Determine the debootstrap script to use
	switch {
	case len(d.DebootstrapScript) > 0:
		// Custom script specified by the user
		d.scriptPath = d.DebootstrapScript

		// When using a custom script we assume any distro-specific workarounds
		// are handled by the user and do not apply the usr-is-merged workarounds.
		d.scriptSuite = ""
	case len(d.DebootstrapSuite) > 0:
		// Explicit parent/debootstrap suite
		if _, err := os.Stat(getDebootstrapScriptPath(d.Suite)); err == nil {
			return fmt.Errorf("suite '%s' has its own debootstrap script; 'debootstrap-suite' should not be set", d.Suite)
		}

		d.scriptSuite = d.DebootstrapSuite
		d.scriptPath = getDebootstrapScriptPath(d.scriptSuite)
		if _, err := os.Stat(d.scriptPath); err != nil {
			return fmt.Errorf("cannot find debootstrap script for debootstrap-suite '%s'", d.scriptSuite)
		}
	default:
		// Automatic behaviour: use debootstrap script for suite if available,
		// otherwise fall back to unstable and suggest setting debootstrap-suite.
		d.scriptSuite = d.Suite
		d.scriptPath = getDebootstrapScriptPath(d.scriptSuite)
		if _, err := os.Stat(d.scriptPath); err != nil {
			d.scriptSuite = "unstable"
			d.scriptPath = getDebootstrapScriptPath(d.scriptSuite)
			if _, err := os.Stat(d.scriptPath); err != nil {
				return errors.New("cannot find debootstrap script for unstable")
			}
		}
	}

	return nil
}

// PreMachine prepares the debootstrap action to run in the fake machine.
func (d *DebootstrapAction) PreMachine(context *debos.Context, m *fakemachine.Machine, _ *[]string) error {
	mounts := d.listOptionFiles(context)

	// Mount configuration files outside of recipes directory
	for _, mount := range mounts {
		m.AddVolume(path.Dir(mount))
	}

	return nil
}

// RunSecondStage runs the second stage of debootstrap in the root filesystem.
func (d *DebootstrapAction) RunSecondStage(context debos.Context) error {
	cmdline := []string{
		"/debootstrap/debootstrap",
		"--no-check-gpg",
		"--second-stage",
	}

	if d.Components != nil {
		s := strings.Join(d.Components, ",")
		cmdline = append(cmdline, fmt.Sprintf("--components=%s", s))
	}

	c := debos.NewChrootCommandForContext(context)
	// Can't use nspawn for debootstrap as it wants to create device nodes
	c.ChrootMethod = debos.ChrootMethodChroot

	err := c.Run("debootstrap (stage 2)", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		catErr := debos.Command{}.Run("debootstrap.log", "cat", log)
		if catErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to show %s: %w", log, catErr))
		}
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
		// Default to not excluding usr-is-merged for anything unknown/new
		// (e.g. >=bookworm and derivatives)
		return false
	}
}

// debootstrapScriptsDir is the directory holding the host debootstrap scripts.
var debootstrapScriptsDir = "/usr/share/debootstrap/scripts"

// getDebootstrapScriptPath returns the path to the host debootstrap script
// for the given suite.
func getDebootstrapScriptPath(script string) string {
	return path.Join(debootstrapScriptsDir, script)
}

func isForeignArch(context *debos.Context) bool {
	/* Only works for amd64, arm64 and riscv64 hosts, which should be enough */
	return context.Architecture != runtime.GOARCH
}

func (d *DebootstrapAction) BuildDebootstrapCommand(context *debos.Context) []string {
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

	if isForeignArch(context) {
		cmdline = append(cmdline, "--foreign")
		cmdline = append(cmdline, fmt.Sprintf("--arch=%s", context.Architecture))
	}

	if d.Variant != "" {
		cmdline = append(cmdline, fmt.Sprintf("--variant=%s", d.Variant))
	}

	// workaround for https://github.com/go-debos/debos/issues/361
	// Apply the workaround only for known-old suites
	if len(d.scriptSuite) > 0 && shouldExcludeUsrIsMerged(d.scriptSuite) {
		log.Printf("excluding usr-is-merged as package is not in suite '%s'\n", d.scriptSuite)
		cmdline = append(cmdline, "--exclude=usr-is-merged")
	}

	cmdline = append(cmdline, d.Suite)
	cmdline = append(cmdline, context.Rootdir)
	cmdline = append(cmdline, d.Mirror)
	// Log here rather than in Verify, which also runs on the host when using
	// fakemachine and would print the messages twice.
	if len(d.DebootstrapScript) > 0 {
		log.Printf("using custom debootstrap script %s\n", d.scriptPath)
	} else if len(d.DebootstrapSuite) == 0 && d.scriptSuite != d.Suite {
		log.Printf("cannot find debootstrap script for suite '%s'; using fallback debootstrap script %s, consider setting 'debootstrap-suite' property\n", d.Suite, d.scriptPath)
	}
	cmdline = append(cmdline, d.scriptPath)

	return cmdline
}

// Run runs the debootstrap action.
func (d *DebootstrapAction) Run(context *debos.Context) error {
	cmdline := d.BuildDebootstrapCommand(context)

	/* Make sure /etc/apt/apt.conf.d exists inside the fakemachine otherwise
	   debootstrap prints a warning about the path not existing. */
	if fakemachine.InMachine() {
		if err := os.MkdirAll(path.Join("/etc/apt/apt.conf.d"), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create /etc/apt/apt.conf.d: %w", err)
		}
	}

	err := debos.Command{}.Run("debootstrap", cmdline...)
	if err != nil {
		log := path.Join(context.Rootdir, "debootstrap/debootstrap.log")
		catErr := debos.Command{}.Run("debootstrap.log", "cat", log)
		if catErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to show %s: %w", log, catErr))
		}

		return err
	}

	if isForeignArch(context) {
		err = d.RunSecondStage(*context)
		if err != nil {
			return err
		}
	}

	/* Cleanup resolv.conf after debootstrap */
	resolvconf := path.Join(context.Rootdir, "/etc/resolv.conf")
	if err = os.Remove(resolvconf); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", resolvconf, err)
	}

	c := debos.NewChrootCommandForContext(*context)

	if err := c.Run("apt clean", "/usr/bin/apt-get", "clean"); err != nil {
		return fmt.Errorf("failed to clean package cache: %w", err)
	}

	return nil
}
