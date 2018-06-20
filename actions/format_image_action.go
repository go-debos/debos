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
	"os/exec"
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

func (i FormatImageAction) format(context debos.DebosContext) error {
	label := fmt.Sprintf("Formatting image")

	cmdline := []string{}
	switch i.FS {
	case "vfat":
		cmdline = append(cmdline, "mkfs.vfat", "-n", i.Label)
	case "btrfs":
		// Force formatting to prevent failure in case if partition was formatted already
		cmdline = append(cmdline, "mkfs.btrfs", "-L", i.Label, "-f")
	case "none":
	default:
		cmdline = append(cmdline, fmt.Sprintf("mkfs.%s", i.FS), "-L", i.Label)
	}

	if len(cmdline) != 0 {
		cmdline = append(cmdline, context.Image)

		cmd := debos.Command{}
		if err := cmd.Run(label, cmdline...); err != nil {
			return err
		}
	}

	return nil
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
	img, err := os.OpenFile(i.ImageName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Couldn't open image file: %v", err)
	}

	err = img.Truncate(i.size)
	if err != nil {
		return fmt.Errorf("Couldn't resize image file: %v", err)
	}

	img.Close()

	loop, err := exec.Command("losetup", "-f", "--show", i.ImageName).Output()
	if err != nil {
		return fmt.Errorf("Failed to setup loop device")
	}
	context.Image = strings.TrimSpace(string(loop[:]))
	i.usingLoop = true

	return nil
}

func (i *FormatImageAction) Run(context *debos.DebosContext) error {
	i.LogStart()

	err := i.format(*context)
	if err != nil {
		return err
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
		exec.Command("losetup", "-d", context.Image).Run()
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
