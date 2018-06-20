/*
FormatImage Action

This action creates an image file and formats it with a filesystem.

Yaml syntax:
 - action: format-image
   imagename: image_name
   imagesize: size
   fs: filesystem
   label: label
   blocksize: 4096

Mandatory properties:

- imagename -- the name of the image file.

- imagesize -- generated image size in human-readable form, examples: 100MB, 1GB, etc.

- fs -- filesystem type used for formatting.

- label -- volume label of the filesystem.

Optional properties:

- blocksize -- size of blocks in bytes, for ext fs only. Valid values are 1024, 2048 and 4096.
*/

package actions

import (
	"fmt"
	"github.com/docker/go-units"
	"github.com/go-debos/fakemachine"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/go-debos/debos"
)

type FormatImageAction struct {
	debos.BaseAction `yaml:",inline"`
	ImageName        string
	ImageSize        string
	FS               string
	Label            string
	BlockSize        int
	size             int64
	usingLoop        bool
}

func (i FormatImageAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine,
	args *[]string) error {
	image, err := m.CreateImage(i.ImageName, i.size)
	if err != nil {
		return err
	}

	context.Image = image
	*args = append(*args, "--internal-image", image)
	return nil
}

func (i *FormatImageAction) PreNoMachine(context *debos.DebosContext) error {
	err := debos.CreateImage(i.ImageName, i.size)
	if (err != nil) {
		return err;
	}

	device, err := debos.SetupLoopDevice(i.ImageName)
	if  (err != nil) {
		return err;
	}

	context.Image = device
	i.usingLoop = true
	return nil
}

func (i *FormatImageAction) Run(context *debos.DebosContext) error {
	i.LogStart()

	err := debos.Format("Formatting image", context.Image, i.FS, i.Label)
	if err != nil {
		return fmt.Errorf("Format failed: %v", err)
	}

	context.ImageMntDir = path.Join(context.Scratchdir, "mnt")
	os.MkdirAll(context.ImageMntDir, 0755)
	err = syscall.Mount(context.Image, context.ImageMntDir, i.FS, 0, "")
	if err != nil {
		return fmt.Errorf("Mount failed: %v", err)
	}

	return nil
}

func (i FormatImageAction) Cleanup(context debos.DebosContext) error {
	syscall.Unmount(context.ImageMntDir, 0)

	if i.usingLoop {
		debos.DetachLoopDevice(context.Image)
	}

	return nil
}

func (i *FormatImageAction) Verify(context *debos.DebosContext) error {
	switch i.FS {
	case "fat32":
		i.FS = "vfat"
	case "":
		return fmt.Errorf("Missing fs type")
	}

	if i.Label == "" {
		return fmt.Errorf("Image without a name")
	}

	size, err := units.FromHumanSize(i.ImageSize)
	if err != nil {
		return fmt.Errorf("Failed to parse image size: %s", i.ImageSize)
	}
	i.size = size

	switch i.BlockSize {
	case 0:
	case 1024, 2048, 4096:
		if !strings.HasPrefix(i.FS, "ext") {
			return fmt.Errorf("Blockize only valid for ext filesystems")
		}
	default:
		return fmt.Errorf("Invalid value for blocksize")
	}

	return nil
}
