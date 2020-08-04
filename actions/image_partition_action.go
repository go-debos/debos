/*
ImagePartition Action

This action creates an image file, partitions it and formats the filesystems.
Mountpoints can be defined so the created partitions can be mounted during the
build, and optionally (but by-default) mounted at boot in the final system. The
mountpoints are sorted on their position in the filesystem hierarchy so the
order in the recipe does not matter.

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
     - name: partition name
	   partlabel: partition label
	   fs: filesystem
	   start: offset
	   end: offset
	   features: list of filesystem features
	   flags: list of flags
	   fsck: bool

Mandatory properties:

- name -- is used for referencing named partition for mount points
configuration (below) and label the filesystem located on this partition. Must be
unique.

- fs -- filesystem type used for formatting.

'none' fs type should be used for partition without filesystem.

- start -- offset from beginning of the disk there the partition starts.

- end -- offset from beginning of the disk there the partition ends.

For 'start' and 'end' properties offset can be written in human readable
form -- '32MB', '1GB' or as disk percentage -- '100%'.

Optional properties:

- partlabel -- label for the partition in the GPT partition table. Defaults
to the `name` property of the partition. May only be used for GPT partitions.

- parttype -- set the partition type in the partition table. The string should
be in a hexadecimal format (2-characters) for msdos partition tables and GUID format
(36-characters) for GPT partition tables. For instance, "82" for msdos sets the
partition type to Linux Swap. Whereas "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f" for
GPT sets the partition type to Linux Swap.
For msdos partition types hex codes see: https://en.wikipedia.org/wiki/Partition_type
For gpt partition type GUIDs see: https://systemd.io/DISCOVERABLE_PARTITIONS/

- features -- list of additional filesystem features which need to be enabled
for partition.

- flags -- list of additional flags for partition compatible with parted(8)
'set' command.

- fsck -- if set to `false` -- then set fs_passno (man fstab) to 0 meaning no filesystem
checks in boot time. By default is set to `true` allowing checks on boot.

Yaml syntax for mount points:

   mountpoints:
     - mountpoint: path
	   partition: partition label
	   options: list of options
	   buildtime: bool

Mandatory properties:

- partition -- partition name for mounting. The partion must exist under `partitions`.

- mountpoint -- path in the target root filesystem where the named partition
should be mounted. Must be unique, only one partition can be mounted per
mountpoint.

Optional properties:

- options -- list of options to be added to appropriate entry in fstab file.

- buildtime -- if set to true then the mountpoint only used during the debos run.
No entry in `/etc/fstab` will be created.
The mountpoints directory will be removed from the image, so it is recommended
to define a `mountpoint` path which is temporary and unique for the image,
for example: `/mnt/temporary_mount`.
Defaults to false.

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
	"gopkg.in/freddierice/go-losetup.v1"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-debos/debos"
)

type Partition struct {
	number    int
	Name      string
	PartLabel string
	PartType  string
	Start     string
	End       string
	FS        string
	Flags     []string
	Features  []string
	Fsck      bool "fsck"
	FSUUID    string
}

type Mountpoint struct {
	Mountpoint string
	Partition  string
	Options    []string
	Buildtime  bool
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
	loopDev          losetup.Device
	usingLoop        bool
}

func (p *Partition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawPartition Partition
	part := rawPartition{Fsck: true}
	if err := unmarshal(&part); err != nil {
		return err
	}
	*p = Partition(part)
	return nil
}

func (i *ImagePartitionAction) generateFSTab(context *debos.DebosContext) error {
	context.ImageFSTab.Reset()

	for _, m := range i.Mountpoints {
		options := []string{"defaults"}
		options = append(options, m.Options...)
		if m.Buildtime == true {
			/* Do not need to add mount point into fstab */
			continue
		}
		if m.part.FSUUID == "" {
			return fmt.Errorf("Missing fs UUID for partition %s!?!", m.part.Name)
		}

		fs_passno := 0

		if m.part.Fsck {
			if m.Mountpoint == "/" {
				fs_passno = 1
			} else {
				fs_passno = 2
			}
		}
		context.ImageFSTab.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t0\t%d\n",
			m.part.FSUUID, m.Mountpoint, m.part.FS,
			strings.Join(options, ","), fs_passno))
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

func (i *ImagePartitionAction) triggerDeviceNodes(context *debos.DebosContext) error {
	err := debos.Command{}.Run("udevadm", "udevadm", "trigger", "--settle", context.Image)
	if err != nil {
		log.Printf("Failed to trigger device nodes")
		return err
	}

	return nil
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

	cmdline := []string{}
	switch p.FS {
	case "vfat":
		cmdline = append(cmdline, "mkfs.vfat", "-F32", "-n", p.Name)
	case "btrfs":
		// Force formatting to prevent failure in case if partition was formatted already
		cmdline = append(cmdline, "mkfs.btrfs", "-L", p.Name, "-f")
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
	case "f2fs":
		cmdline = append(cmdline, "mkfs.f2fs", "-l", p.Name)
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
	case "hfs":
		cmdline = append(cmdline, "mkfs.hfs", "-h", "-v", p.Name)
	case "hfsplus":
		cmdline = append(cmdline, "mkfs.hfsplus", "-v", p.Name)
	case "hfsx":
		cmdline = append(cmdline, "mkfs.hfsplus", "-s", "-v", p.Name)
		// hfsx is case-insensitive hfs+, should be treated as "normal" hfs+ from now on
		p.FS = "hfsplus"
	case "none":
	default:
		cmdline = append(cmdline, fmt.Sprintf("mkfs.%s", p.FS), "-L", p.Name)
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
	}

	if len(cmdline) != 0 {
		cmdline = append(cmdline, path)

		cmd := debos.Command{}
		if err := cmd.Run(label, cmdline...); err != nil {
			return err
		}
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

	img, err := os.OpenFile(i.ImageName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Couldn't open image file: %v", err)
	}

	err = img.Truncate(i.size)
	if err != nil {
		return fmt.Errorf("Couldn't resize image file: %v", err)
	}

	img.Close()

	i.loopDev, err = losetup.Attach(i.ImageName, 0, false)
	if err != nil {
		return fmt.Errorf("Failed to setup loop device")
	}
	context.Image = i.loopDev.Path()
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
	 * After the fd is closed the kernel needs to be informed of partition table
	 * changes (defer calls are executed in LIFO order) */
	defer i.triggerDeviceNodes(context)
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

		if p.PartLabel == "" {
			p.PartLabel = p.Name
		}

		var name string
		if i.PartitionType == "gpt" {
			name = p.PartLabel
		} else {
			name = "primary"
		}

		command := []string{"parted", "-a", "none", "-s", "--", context.Image, "mkpart", name}
		switch p.FS {
		case "vfat":
			command = append(command, "fat32")
		case "hfsplus":
			command = append(command, "hfs+")
		case "f2fs":
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

		if p.PartType != "" {
			err = debos.Command{}.Run("sfdisk", "sfdisk", "--part-type", context.Image, fmt.Sprintf("%d", p.number), p.PartType)
			if err != nil {
				return err
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

	// sort mountpoints based on position in filesystem hierarchy
	sort.SliceStable(i.Mountpoints, func(a, b int) bool {
		mntA := i.Mountpoints[a].Mountpoint
		mntB := i.Mountpoints[b].Mountpoint

		// root should always be mounted first
		if (mntA == "/") {
			return true
		}
		if (mntB == "/") {
			return false
		}

		return strings.Count(mntA, "/") < strings.Count(mntB, "/")
	})

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
		err := syscall.Unmount(mntpath, 0)
		if err != nil {
			log.Printf("Warning: Failed to get unmount %s: %s", m.Mountpoint, err)
			log.Printf("Unmount failure can cause images being incomplete!")
			return err
		}
		if m.Buildtime == true {
			if err = os.Remove(mntpath); err != nil {
				log.Printf("Failed to remove temporary mount point %s: %s", m.Mountpoint, err)

				if err.(*os.PathError).Err.Error() == "read-only file system" {
					continue
				}

				return err
			}
		}
	}

	if i.usingLoop {
		err := i.loopDev.Detach()
		if err != nil {
			log.Printf("WARNING: Failed to detach loop device: %s", err)
			return err
		}

		for t := 0; t < 60; t++ {
			err = i.loopDev.Remove()
			if err == nil {
				break
			}
			log.Printf("Loop dev couldn't remove %s, waiting", err)
			time.Sleep(time.Second)
		}

		if err != nil {
			log.Printf("WARNING: Failed to remove loop device: %s", err)
			return err
		}
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

		// check for duplicate partition names
		for j := idx + 1; j < len(i.Partitions); j++ {
			if i.Partitions[j].Name == p.Name {
				return fmt.Errorf("Partition %s already exists", p.Name)
			}
		}

		if i.PartitionType != "gpt" && p.PartLabel != "" {
			return fmt.Errorf("Can only set partition partlabel on GPT filesystem")
		}

		if p.PartType != "" {
			var partTypeLen int
			switch i.PartitionType {
			case "gpt":
				partTypeLen = 36
			case "msdos":
				partTypeLen = 2
			}
			if len(p.PartType) != partTypeLen {
				return fmt.Errorf("incorrect partition type for %s, should be %d characters", p.Name, partTypeLen)
			}
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

		// check for duplicate mountpoints
		for j := idx + 1; j < len(i.Mountpoints); j++ {
			if i.Mountpoints[j].Mountpoint == m.Mountpoint {
				return fmt.Errorf("Mountpoint %s already exists", m.Mountpoint)
			}
		}

		for pidx, _ := range i.Partitions {
			p := &i.Partitions[pidx]
			if m.Partition == p.Name {
				m.part = p
				break
			}
		}
		if m.part == nil {
			return fmt.Errorf("Couldn't find partition for %s", m.Mountpoint)
		}
	}

	size, err := units.FromHumanSize(i.ImageSize)
	if err != nil {
		return fmt.Errorf("Failed to parse image size: %s", i.ImageSize)
	}

	i.size = size
	return nil
}
