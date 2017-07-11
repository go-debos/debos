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
}

func (i SetupImage) PreMachine(context *YaibContext, m *fakemachine.Machine,
	args *[]string) {
	var size int64
	size, err := units.FromHumanSize(i.ImageSize)

	if err != nil {
		log.Fatal("Failed to parse image size: %s", i.ImageSize)
	}

	if context.image != "" {
		log.Fatal("Cannot support two images")
	}

	context.image = "/dev/vda"

	m.CreateImage(i.ImageName, size)
	*args = append(*args, "--internal-image", "/dev/vda")
}

func formatPartition(p *Partition, context YaibContext) {
	label := fmt.Sprintf("Formatting partition %d", p.number)
	path := fmt.Sprintf("%s%d", context.image, p.number)
	var mkfs string

	options := []string{}
	switch p.FS {
	case "fat32":
		mkfs = "mkfs.vfat"
		options = append(options, "-n", p.Name)
	default:
		options = append(options, "-L", p.Name)
		mkfs = fmt.Sprintf("mkfs.%s", p.FS)
	}
	options = append(options, path)

	RunCommand(label, mkfs, options...)

	uuid, err := exec.Command("blkid", "-o", "value", "-s", "UUID", "-p", "-c", "none", path).Output()
	if err != nil {
		log.Fatal("Failed to get uuid")
	}
	p.FSUUID = strings.TrimSpace(string(uuid[:]))
}

func (i SetupImage) generateFSTab(context *YaibContext) {
	fstab := path.Join(context.rootdir, "etc/fstab")
	f, err := os.OpenFile(fstab, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		log.Fatalf("Couldn't open fstab: %v", err)
	}

	for _, m := range i.Mountpoints {
		options := []string{"defaults"}
		options = append(options, m.Options...)
		f.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t0\t0\n",
			m.part.FSUUID, m.Mountpoint, m.part.FS,
			strings.Join(options, ",")))
	}
	f.Close()
}

func (i SetupImage) updateKernelCmdline(context *YaibContext) {
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

func (i SetupImage) Run(context *YaibContext) {
	RunCommand("parted", "parted", "-s", context.image, "mklabel", i.PartitionType)
	for idx, _ := range i.Partitions {
		p := &i.Partitions[idx]
		var name string
		if i.PartitionType == "gpt" {
			name = p.Name
		} else {
			name = "primary"
		}
		RunCommand("parted", "parted", "-a", "none", "-s", context.image, "mkpart",
			name, p.FS, p.Start, p.End)
		formatPartition(p, *context)
	}

	context.imageMntDir = "/scratch/mnt"
	os.MkdirAll(context.imageMntDir, 755)
	for _, m := range i.Mountpoints {
		dev := fmt.Sprintf("%s%d", context.image, m.part.number)
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
			log.Fatalf("%s mount failed: %v", m.part.Name, err)
		}
	}

	/* Copying files is actually silly hard, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	RunCommand("Deploy to image", "cp", "-a", context.rootdir+"/.", context.imageMntDir)
	context.rootdir = context.imageMntDir

	i.generateFSTab(context)
	i.updateKernelCmdline(context)
}

func (i SetupImage) Cleanup(context YaibContext) {
	for idx := len(i.Mountpoints) - 1; idx >= 0; idx-- {
		m := i.Mountpoints[idx]
		mntpath := path.Join(context.imageMntDir, m.Mountpoint)
		syscall.Unmount(mntpath, 0)
	}
}

func (i *SetupImage) Verify(context YaibContext) {
	num := 1
	for idx, _ := range i.Partitions {
		p := &i.Partitions[idx]
		p.number = num
		num++
		if p.Name == "" {
			log.Fatal("Partition without a name")
		}
		if p.Start == "" {
			log.Fatalf("Partition %s missing start", p.Name)
		}
		if p.End == "" {
			log.Fatalf("Partition %s missing end", p.Name)
		}

		if p.FS == "" {
			log.Fatalf("Partition %s missing fs type", p.Name)
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
			log.Fatalf("Couldn't fount partition for %s", m.Mountpoint)
		}
	}
}
