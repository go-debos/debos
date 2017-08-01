package main

import (
	"fmt"
	"github.com/docker/go-units"
	"github.com/sjoerdsimons/fakemachine"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
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

type MountPoint struct {
	Mountpoint string
	Partition  string
	Options    []string
	part       *Partition
}

type SetupImage struct {
	*BaseAction
	ImageName     string
	ImageSize     string
	PartitionType string
	Partitions    []Partition
	Mountpoints   []MountPoint
	size          int64
	usingLoop     bool
}

func (i SetupImage) getPartitionDevice(number int, context YaibContext) string {
	/* If the iamge device has a digit as the last character, the partition
	 * suffix is p<number> else it's just <number> */
	last := context.image[len(context.image)-1]
	if last >= '0' && last <= '9' {
		return fmt.Sprintf("%sp%d", context.image, number)
	} else {
		return fmt.Sprintf("%s%d", context.image, number)
	}
}

func (i SetupImage) PreMachine(context *YaibContext, m *fakemachine.Machine,
	args *[]string) error {
	err := m.CreateImage(i.ImageName, i.size)
	if err != nil {
		return err
	}

	context.image = "/dev/vda"
	*args = append(*args, "--internal-image", "/dev/vda")
	return nil
}

func (i SetupImage) formatPartition(p *Partition, context YaibContext) error {
	label := fmt.Sprintf("Formatting partition %d", p.number)
	path := i.getPartitionDevice(p.number, context)

	cmdline := []string{}
	switch p.FS {
	case "fat32":
		cmdline = append(cmdline, "mkfs.vfat", "-n", p.Name)
	default:
		cmdline = append(cmdline, fmt.Sprintf("mkfs.%s", p.FS), "-L", p.Name)
	}
	cmdline = append(cmdline, path)

	Command{}.Run(label, cmdline...)

	uuid, err := exec.Command("blkid", "-o", "value", "-s", "UUID", "-p", "-c", "none", path).Output()
	if err != nil {
		return fmt.Errorf("Failed to get uuid: %s", err)
	}
	p.FSUUID = strings.TrimSpace(string(uuid[:]))

	return nil
}

func (i SetupImage) generateFSTab(context *YaibContext) error {
	err := os.MkdirAll(path.Join(context.rootdir, "etc"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc in image: %v", err)
	}

	fstab := path.Join(context.rootdir, "etc/fstab")
	f, err := os.OpenFile(fstab, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		return fmt.Errorf("Couldn't open fstab: %v", err)
	}

	for _, m := range i.Mountpoints {
		options := []string{"defaults"}
		options = append(options, m.Options...)
		f.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t0\t0\n",
			m.part.FSUUID, m.Mountpoint, m.part.FS,
			strings.Join(options, ",")))
	}
	f.Close()

	return nil
}

func (i SetupImage) updateKernelCmdline(context *YaibContext) {
	err := os.MkdirAll(path.Join(context.rootdir, "etc", "kernel"), 0755)
	if err != nil {
		log.Fatalf("Couldn't create etc/kernel in image: %v", err)
	}
	path := path.Join(context.rootdir, "etc/kernel/cmdline")
	current, _ := ioutil.ReadFile(path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		log.Fatalf("Couldn't open kernel cmdline: %v", err)
	}

	for _, m := range i.Mountpoints {
		if m.Mountpoint == "/" {
			cmdline := fmt.Sprintf("root=UUID=%s %s\n", m.part.FSUUID,
				strings.TrimSpace(string(current)))
			f.WriteString(cmdline)
			break
		}
	}
	f.Close()
}

func (i SetupImage) PreNoMachine(context *YaibContext) error {

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
	context.image = strings.TrimSpace(string(loop[:]))
	i.usingLoop = true

	return nil
}

func (i SetupImage) Run(context *YaibContext) error {
	err := Command{}.Run("parted", "parted", "-s", context.image, "mklabel", i.PartitionType)
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
		err = Command{}.Run("parted", "parted", "-a", "none", "-s", context.image, "mkpart",
			name, p.FS, p.Start, p.End)
		if err != nil {
			return err
		}

		if p.Flags != nil {
			for _, flag := range p.Flags {
				err = Command{}.Run("parted", "parted", "-s", context.image, "set",
					fmt.Sprintf("%d", p.number), flag, "on")
				if err != nil {
					return err
				}
			}
		}

		err = i.formatPartition(p, *context)
		if err != nil {
			return err
		}
	}

	context.imageMntDir = path.Join(context.scratchdir, "mnt")
	os.MkdirAll(context.imageMntDir, 755)
	for _, m := range i.Mountpoints {
		dev := i.getPartitionDevice(m.part.number, *context)
		mntpath := path.Join(context.imageMntDir, m.Mountpoint)
		os.MkdirAll(mntpath, 755)
		var fs string
		switch m.part.FS {
		case "fat32":
			fs = "vfat"
		default:
			fs = m.part.FS
		}
		err := syscall.Mount(dev, mntpath, fs, 0, "")
		if err != nil {
			return fmt.Errorf("%s mount failed: %v", m.part.Name, err)
		}
	}

	/* Copying files is actually silly hard, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	err = Command{}.Run("Deploy to image", "cp", "-a", context.rootdir+"/.", context.imageMntDir)
	if err != nil {
		return fmt.Errorf("rootfs deploy failed: %v", err)
	}
	context.rootdir = context.imageMntDir

	i.generateFSTab(context)
	i.updateKernelCmdline(context)

	return nil
}

func (i SetupImage) Cleanup(context YaibContext) error {
	for idx := len(i.Mountpoints) - 1; idx >= 0; idx-- {
		m := i.Mountpoints[idx]
		mntpath := path.Join(context.imageMntDir, m.Mountpoint)
		syscall.Unmount(mntpath, 0)
	}

	if i.usingLoop {
		exec.Command("losetup", "-d", context.image).Run()
	}

	return nil
}

func (i *SetupImage) Verify(context *YaibContext) error {
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

		if p.FS == "" {
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
