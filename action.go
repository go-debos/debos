package debos

import (
	"bytes"
	"github.com/go-debos/fakemachine"
	"log"
)

// Mapping from partition name as configured in the image-partition action to
// device path for usage by other actions
type Partition struct {
	Name       string
	DevicePath string
}

type DebosContext struct {
	Scratchdir      string
	Rootdir         string
	Artifactdir     string
	Downloaddir     string
	Image           string
	ImagePartitions []Partition
	ImageMntDir     string
	ImageFSTab      bytes.Buffer // Fstab as per partitioning
	ImageKernelRoot string       // Kernel cmdline root= snippet for the / of the image
	RecipeDir       string
	Architecture    string
	DebugShell      string
	Origins         map[string]string
}

type Action interface {
	/* FIXME verify should probably be prepare or somesuch */
	Verify(context *DebosContext) error
	PreMachine(context *DebosContext, m *fakemachine.Machine, args *[]string) error
	PreNoMachine(context *DebosContext) error
	Run(context *DebosContext) error
	Cleanup(context DebosContext) error
	PostMachine(context DebosContext) error
	String() string
}

type BaseAction struct {
	Action      string
	Description string
}

func (b *BaseAction) LogStart() {
	log.Printf("==== %s ====\n", b)
}

func (b *BaseAction) Verify(context *DebosContext) error { return nil }
func (b *BaseAction) PreMachine(context *DebosContext,
	m *fakemachine.Machine,
	args *[]string) error {
	return nil
}
func (b *BaseAction) PreNoMachine(context *DebosContext) error { return nil }
func (b *BaseAction) Run(context *DebosContext) error          { return nil }
func (b *BaseAction) Cleanup(context DebosContext) error       { return nil }
func (b *BaseAction) PostMachine(context DebosContext) error   { return nil }
func (b *BaseAction) String() string {
	if b.Description == "" {
		return b.Action
	}
	return b.Description
}
