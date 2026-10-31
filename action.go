// Package debos provides the building blocks of the debos image builder:
// the recipe Action interface and its Context, command execution in a chroot
// or container, archive unpacking and filesystem helpers.
package debos

import (
	"bytes"

	"github.com/go-debos/fakemachine"
)

// State is the overall outcome of a debos run.
type State int

// Possible values of State.
const (
	Success State = iota
	Failed
)

// Partition maps a partition name, as configured in the image-partition
// action, to a device path for usage by other actions.
type Partition struct {
	Name       string
	DevicePath string
}

// CommonContext holds the state shared by every Context of a run, including
// the ones created for sub-recipes.
type CommonContext struct {
	Scratchdir      string
	Rootdir         string
	Artifactdir     string
	Downloaddir     string
	Image           string
	ImagePartitions []Partition
	ImageMntDir     string
	ImageFSTab      bytes.Buffer // Fstab as per partitioning
	ImageKernelRoot string       // Kernel cmdline root= snippet for the / of the image
	DebugShell      string
	Origins         map[string]string
	State           State
	EnvironVars     map[string]string
	PrintRecipe     bool
	Verbose         bool
}

// Context is the per-recipe view of a run: the shared CommonContext plus the
// recipe directory, target architecture and sector size.
type Context struct {
	*CommonContext

	RecipeDir    string
	Architecture string
	SectorSize   int
}

// Origin returns the path of the named origin and whether it exists. The
// "recipe" origin is always defined and refers to the recipe directory.
func (c *Context) Origin(o string) (string, bool) {
	if o == "recipe" {
		return c.RecipeDir, true
	}
	path, found := c.Origins[o]

	return path, found
}

// Action is a single step of a recipe.
type Action interface {
	/* FIXME verify should probably be prepare or somesuch */
	Verify(context *Context) error
	PreMachine(context *Context, m *fakemachine.Machine, args *[]string) error
	PreNoMachine(context *Context) error
	Run(context *Context) error
	// Cleanup() method gets called only if the Run for an action
	// was started and in the same machine (host or fake) as Run has run
	Cleanup(context *Context) error
	PostMachine(context *Context) error
	// PostMachineCleanup() gets called for all actions if Pre*Machine() method
	// has run for Action. This method is always executed on the host with user's permissions.
	PostMachineCleanup(context *Context) error
	String() string
}

// BaseAction provides no-op implementations of Action for actions to embed.
type BaseAction struct {
	Action      string
	Description string
}

// Verify does nothing.
func (b *BaseAction) Verify(_ *Context) error { return nil }

// PreMachine does nothing.
func (b *BaseAction) PreMachine(_ *Context,
	_ *fakemachine.Machine,
	_ *[]string,
) error {
	return nil
}

// PreNoMachine does nothing.
func (b *BaseAction) PreNoMachine(_ *Context) error { return nil }

// Run does nothing.
func (b *BaseAction) Run(_ *Context) error { return nil }

// Cleanup does nothing.
func (b *BaseAction) Cleanup(_ *Context) error { return nil }

// PostMachine does nothing.
func (b *BaseAction) PostMachine(_ *Context) error { return nil }

// PostMachineCleanup does nothing.
func (b *BaseAction) PostMachineCleanup(_ *Context) error { return nil }

// String returns the description of the action, or its name if it has none.
func (b *BaseAction) String() string {
	if b.Description == "" {
		return b.Action
	}

	return b.Description
}
