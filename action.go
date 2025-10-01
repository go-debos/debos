package debos

import (
	"bytes"
	"github.com/go-debos/fakemachine"
)

type State int

// Represent the current state of Debos
const (
	Success State = iota
	Failed
)

// Mapping from partition name as configured in the image-partition action to
// device path for usage by other actions
type Partition struct {
	Name       string
	DevicePath string
}

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

type Context struct {
	*CommonContext
	RecipeDir    string
	Architecture string
	SectorSize   int
}

func (c *Context) Origin(o string) (string, bool) {
	if o == "recipe" {
		return c.RecipeDir, true
	}
	path, found := c.Origins[o]
	return path, found
}

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

type BaseAction struct {
	Action      string
	Description string
}

func (b *BaseAction) Verify(_ *Context) error { return nil }
func (b *BaseAction) PreMachine(_ *Context,
	_ *fakemachine.Machine,
	_ *[]string) error {
	return nil
}
func (b *BaseAction) PreNoMachine(_ *Context) error       { return nil }
func (b *BaseAction) Run(_ *Context) error                { return nil }
func (b *BaseAction) Cleanup(_ *Context) error            { return nil }
func (b *BaseAction) PostMachine(_ *Context) error        { return nil }
func (b *BaseAction) PostMachineCleanup(_ *Context) error { return nil }
func (b *BaseAction) String() string {
	if b.Description == "" {
		return b.Action
	}
	return b.Description
}
