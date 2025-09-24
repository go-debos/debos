package debos

import (
	"bytes"
	"log"

	"github.com/go-debos/fakemachine"
)

type DebosState int

// Represent the current state of Debos
const (
	Success DebosState = iota
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
	State           DebosState
	EnvironVars     map[string]string
	PrintRecipe     bool
	Verbose         bool
}

type DebosContext struct {
	*CommonContext
	RecipeDir    string
	Architecture string
	SectorSize   int
}

func (c *DebosContext) Origin(o string) (string, bool) {
	if o == "recipe" {
		return c.RecipeDir, true
	} else {
		path, found := c.Origins[o]
		return path, found
	}
}

func HandleError(context *DebosContext, err error, a Action, stage string) bool {
	if err == nil {
		return false
	}

	context.State = Failed
	log.Printf("Action `%s` failed at stage %s, error: %s", a, stage, err)
	DebugShell(*context)
	return true
}

type Action interface {
	/* FIXME verify should probably be prepare or somesuch */
	Verify(context *DebosContext) error
	PreMachine(context *DebosContext, m *fakemachine.Machine, args *[]string) error
	PreNoMachine(context *DebosContext) error
	Run(context *DebosContext) error
	// Cleanup() method gets called only if the Run for an action
	// was started and in the same machine (host or fake) as Run has run
	Cleanup(context *DebosContext) error
	PostMachine(context *DebosContext) error
	// PostMachineCleanup() gets called for all actions if Pre*Machine() method
	// has run for Action. This method is always executed on the host with user's permissions.
	PostMachineCleanup(context *DebosContext) error
	String() string
}

type BaseAction struct {
	Action      string
	Description string
}

func (b *BaseAction) Verify(context *DebosContext) error { return nil }
func (b *BaseAction) PreMachine(context *DebosContext,
	m *fakemachine.Machine,
	args *[]string) error {
	return nil
}
func (b *BaseAction) PreNoMachine(context *DebosContext) error       { return nil }
func (b *BaseAction) Run(context *DebosContext) error                { return nil }
func (b *BaseAction) Cleanup(context *DebosContext) error            { return nil }
func (b *BaseAction) PostMachine(context *DebosContext) error        { return nil }
func (b *BaseAction) PostMachineCleanup(context *DebosContext) error { return nil }
func (b *BaseAction) String() string {
	if b.Description == "" {
		return b.Action
	}
	return b.Description
}
