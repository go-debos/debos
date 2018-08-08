/*
ImagePartition Action

This action creates an image file, partitions it and formats the filesystems.

Yaml syntax:
 - action: image-partition
   imagename: image_name
   imagesize: size
   partitiontype: gpt
   gpt_gap: offset
   partitions:
     <list of partitions>
   mountpoints:
     <list of mount points>

Mandatory properties:

- imagename -- the name of the image file.

- imagesize -- generated image size in human-readable form, examples: 100MB, 1GB, etc.

- partitiontype -- partition table type. Currently only 'gpt' and 'msdos'
partition tables are supported.

- gpt_gap -- shifting GPT allow to use this gap for bootloaders, for example if
U-Boot intersects with original GPT placement.
Only works if parted supports an extra argument to mklabel to specify the gpt offset.

- partitions -- list of partitions, at least one partition is needed.
Partition properties are described below.

- mountpoints -- list of mount points for partitions.
Properties for mount points are described below.

Yaml syntax for partitions:

   partitions:
     - name: label
	   name: partition name
	   fs: filesystem
	   start: offset
	   end: offset
	   flags: list of flags

Mandatory properties:

- name -- is used for referencing named partition for mount points
configuration (below) and label the filesystem located on this partition.

- fs -- filesystem type used for formatting.

'none' fs type should be used for partition without filesystem.

- start -- offset from beginning of the disk there the partition starts.

- end -- offset from beginning of the disk there the partition ends.

For 'start' and 'end' properties offset can be written in human readable
form -- '32MB', '1GB' or as disk percentage -- '100%'.

Optional properties:

- flags -- list of additional flags for partition compatible with parted(8)
'set' command.

Yaml syntax for mount points:

   mountpoints:
     - mountpoint: path
	   partition: partition label
	   options: list of options

Mandatory properties:

- partition -- partition name for mounting.

- mountpoint -- path in the target root filesystem where the named partition
should be mounted.

Optional properties:

- options -- list of options to be added to appropriate entry in fstab file.

Layout example for Raspberry PI 3:

 - action: image-partition
   imagename: "debian-rpi3.img"
   imagesize: 1GB
   partitiontype: msdos
   mountpoints:
     - mountpoint: /
       partition: root
     - mountpoint: /boot/firmware
       partition: firmware
       options: [ x-systemd.automount ]
   partitions:
     - name: firmware
       fs: vfat
       start: 0%
       end: 64MB
     - name: root
       fs: ext4
       start: 64MB
       end: 100%
       flags: [ boot ]
*/
package actions

import (
	"errors"
	"fmt"
	"github.com/docker/go-units"
	"github.com/go-debos/fakemachine"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-debos/debos"
)

type Partition struct {
	number int
	Name   string
	Start  string
	End    string
	FS     string
	Flags  []string
	FSUUID string
}

type Mountpoint struct {
	Mountpoint string
	Partition  string
	Options    []string
	part       *Partition
}

type ImagePartitionAction struct {
	debos.BaseAction `yaml:",inline"`
	ImageName        string
	ImageSize        string
	PartitionType    string
	GptGap           string "gpt_gap"
	Partitions       []Partition
	Mountpoints      []Mountpoint
	size             int64
	usingLoop        bool
}

func (i *ImagePartitionAction) generateFSTab(context *debos.DebosContext) error {
	context.ImageFSTab.Reset()

	for _, m := range i.Mountpoints {
		options := []string{"defaults"}
		options = append(options, m.Options...)
		if m.part.FSUUID == "" {
			return fmt.Errorf("Missing fs UUID for partition %s!?!", m.part.Name)
		}
		context.ImageFSTab.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t0\t0\n",
			m.part.FSUUID, m.Mountpoint, m.part.FS,
			strings.Join(options, ",")))
	}

	return nil
}

func (i *ImagePartitionAction) generateKernelRoot(context *debos.DebosContext) error {
	for _, m := range i.Mountpoints {
		if m.Mountpoint == "/" {
			if m.part.FSUUID == "" {
				return errors.New("No fs UUID for root partition !?!")
			}
			context.ImageKernelRoot = fmt.Sprintf("root=UUID=%s", m.part.FSUUID)
			break
		}
	}

	return nil
}

func (i ImagePartitionAction) getPartitionDevice(number int, context debos.DebosContext) string {
	/* Always look up canonical device as udev might not generate the by-id
	 * symlinks while there is an flock on /dev/vda */
	device, _ := filepath.EvalSymlinks(context.Image)

	suffix := "p"
	/* Check partition naming first: if used 'by-id'i naming convention */
	if strings.Contains(device, "/disk/by-id/") {
		suffix = "-part"
	}

	/* If the iamge device has a digit as the last character, the partition
	 * suffix is p<number> else it's just <number> */
	last := device[len(device)-1]
	if last >= '0' && last <= '9' {
		return fmt.Sprintf("%s%s%d", device, suffix, number)
	} else {
		return fmt.Sprintf("%s%d", device, number)
	}
}

func (i ImagePartitionAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine,
	args *[]string) error {
	image, err := m.CreateImage(i.ImageName, i.size)
	if err != nil {
		return err
	}

	context.Image = image
	*args = append(*args, "--internal-image", image)
	return nil
}

func (i ImagePartitionAction) formatPartition(p *Partition, context debos.DebosContext) error {
	label := fmt.Sprintf("Formatting partition %d", p.number)
	path := i.getPartitionDevice(p.number, context)

	err := debos.Format(label, path, p.FS, p.Name)
	if err != nil {
		return fmt.Errorf("Format failed: %v", err)
	}

	if p.FS != "none" {
		uuid, err := exec.Command("blkid", "-o", "value", "-s", "UUID", "-p", "-c", "none", path).Output()
		if err != nil {
			return fmt.Errorf("Failed to get uuid: %s", err)
		}
		p.FSUUID = strings.TrimSpace(string(uuid[:]))
	}

	return nil
}

func (i *ImagePartitionAction) PreNoMachine(context *debos.DebosContext) error {
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

func (i ImagePartitionAction) Run(context *debos.DebosContext) error {
	i.LogStart()

	/* Exclusively Lock image device file to prevent udev from triggering
	 * partition rescans, which cause confusion as some time asynchronously the
	 * partition device might disappear and reappear due to that! */
	imageFD, err := os.Open(context.Image)
	if err != nil {
		return err
	}
	/* Defer will keep the fd open until the function returns, at which points
	 * the filesystems will have been mounted protecting from more udev funnyness
	 */
	defer imageFD.Close()

	err = syscall.Flock(int(imageFD.Fd()), syscall.LOCK_EX)
	if err != nil {
		return err
	}

	command := []string{"parted", "-s", context.Image, "mklabel", i.PartitionType}
	if len(i.GptGap) > 0 {
		command = append(command, i.GptGap)
	}
	err = debos.Command{}.Run("parted", command...)
	if err != nil {
		return err
	}
	for idx, _ := range i.Partitions {
		p := &i.Partitions[idx]
		var name string
		if i.PartitionType == "gpt" {
			name = p.Name
		} else {
			name = "primary"
		}

		command := []string{"parted", "-a", "none", "-s", "--", context.Image, "mkpart", name}
		switch p.FS {
		case "vfat":
			command = append(command, "fat32")
		case "none":
		default:
			command = append(command, p.FS)
		}
		command = append(command, p.Start, p.End)

		err = debos.Command{}.Run("parted", command...)
		if err != nil {
			return err
		}

		if p.Flags != nil {
			for _, flag := range p.Flags {
				err = debos.Command{}.Run("parted", "parted", "-s", context.Image, "set",
					fmt.Sprintf("%d", p.number), flag, "on")
				if err != nil {
					return err
				}
			}
		}

		devicePath := i.getPartitionDevice(p.number, *context)

		err = i.formatPartition(p, *context)
		if err != nil {
			return err
		}

		context.ImagePartitions = append(context.ImagePartitions,
			debos.Partition{p.Name, devicePath})
	}

	context.ImageMntDir = path.Join(context.Scratchdir, "mnt")
	os.MkdirAll(context.ImageMntDir, 0755)
	for _, m := range i.Mountpoints {
		dev := i.getPartitionDevice(m.part.number, *context)
		mntpath := path.Join(context.ImageMntDir, m.Mountpoint)
		os.MkdirAll(mntpath, 0755)
		err := syscall.Mount(dev, mntpath, m.part.FS, 0, "")
		if err != nil {
			return fmt.Errorf("%s mount failed: %v", m.part.Name, err)
		}
	}

	err = i.generateFSTab(context)
	if err != nil {
		return err
	}

	err = i.generateKernelRoot(context)
	if err != nil {
		return err
	}

	return nil
}

func (i ImagePartitionAction) Cleanup(context *debos.DebosContext) error {
	for idx := len(i.Mountpoints) - 1; idx >= 0; idx-- {
		m := i.Mountpoints[idx]
		mntpath := path.Join(context.ImageMntDir, m.Mountpoint)
		syscall.Unmount(mntpath, 0)
	}

	if i.usingLoop {
		debos.DetachLoopDevice(context.Image)
	}

	return nil
}

func (i ImagePartitionAction) PostMachineCleanup(context *debos.DebosContext) error {
	image := path.Join(context.Artifactdir, i.ImageName)
	/* Remove the image in case of any action failure */
	if context.State != debos.Success {
		if _, err := os.Stat(image); !os.IsNotExist(err) {
			if err = os.Remove(image); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *ImagePartitionAction) Verify(context *debos.DebosContext) error {
	if len(i.GptGap) > 0 {
		log.Println("WARNING: special version of parted is needed for 'gpt_gap' option")
		if i.PartitionType != "gpt" {
			return fmt.Errorf("gpt_gap property could be used only with 'gpt' label")
		}
		// Just check if it contains correct value
		_, err := units.FromHumanSize(i.GptGap)
		if err != nil {
			return fmt.Errorf("Failed to parse GPT offset: %s", i.GptGap)
		}
	}

	num := 1
	for idx, _ := range i.Partitions {
		p := &i.Partitions[idx]
		p.number = num
		num++
		if p.Name == "" {
			return fmt.Errorf("Partition without a name")
		}
		if p.Start == "" {
			return fmt.Errorf("Partition %s missing start", p.Name)
		}
		if p.End == "" {
			return fmt.Errorf("Partition %s missing end", p.Name)
		}

		switch p.FS {
		case "fat32":
			p.FS = "vfat"
		case "":
			return fmt.Errorf("Partition %s missing fs type", p.Name)
		}
	}

	for idx, _ := range i.Mountpoints {
		m := &i.Mountpoints[idx]
		for pidx, _ := range i.Partitions {
			p := &i.Partitions[pidx]
			if m.Partition == p.Name {
				m.part = p
				break
			}
		}
		if m.part == nil {
			return fmt.Errorf("Couldn't fount partition for %s", m.Mountpoint)
		}
	}

	size, err := units.FromHumanSize(i.ImageSize)
	if err != nil {
		return fmt.Errorf("Failed to parse image size: %s", i.ImageSize)
	}

	i.size = size
	return nil
}
