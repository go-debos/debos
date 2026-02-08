/*
ImagePartition Action

This action creates an image file, partitions it and formats the filesystems.
Mountpoints can be defined so the created partitions can be mounted during the
build, and optionally (but by-default) mounted at boot in the final system. The
mountpoints are sorted on their position in the filesystem hierarchy so the
order in the recipe does not matter.

	# Yaml syntax:
	- action: image-partition
	  imagename: image_name
	  imagesize: size
	  partitiontype: gpt
	  diskid: string
	  gpt_gap: offset
	  partitions:
	    <list of partitions>
	  mountpoints:
	    <list of mount points>

Mandatory properties:

- imagename -- the name of the image file, relative to the artifact directory.

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

Optional properties:

- diskid -- disk unique identifier string. For 'gpt' partition table, 'diskid'
should be in GUID format (e.g.: '00002222-4444-6666-AAAA-BBBBCCCCFFFF' where each
character is an hexadecimal digit). For 'msdos' partition table, 'diskid' should be
a 32 bits hexadecimal number (e.g. '1234ABCD' without any dash separator).

	   # Yaml syntax for partitions:
	   partitions:
	     - name: partition name
		   partlabel: partition label
		   fs: filesystem
		   fslabel: filesystem label
		   start: offset
		   end: offset
		   features: list of filesystem features
		   extendedoptions: list of filesystem extended options
		   flags: list of flags
		   fsck: bool
		   fsuuid: string
		   parttype: string
		   partuuid: string
		   partattrs: list of partition attribute bits to set

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

- fslabel -- label for the filesystem. Defaults
to the `name` property of the partition. The filesystem label can be up to 11
characters long for {v}fat{12|16|32}, 16 characters long for ext2/3/4, 255
characters long for btrfs, 512 characters long for hfs/hfsplus and 12 characters
long for xfs.

- parttype -- set the partition type in the partition table. The string should
be in a hexadecimal format (2-characters) for msdos partition tables and GUID format
(36-characters) for GPT partition tables. For GPT, a partition type identifier may
also be used. For instance, "82" for msdos sets the partition type to Linux Swap.
Whereas "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f" or "swap" for GPT sets the partition
type to Linux Swap.
For msdos partition types hex codes see: https://en.wikipedia.org/wiki/Partition_type
For gpt partition type GUIDs see: https://systemd.io/DISCOVERABLE_PARTITIONS/ and
https://www.freedesktop.org/software/systemd/man/latest/repart.d.html for supported
type identifiers.

- features -- list of additional filesystem features which need to be enabled
for partition.

- flags -- list of additional flags for partition compatible with parted(8)
'set' command.

- partattrs -- list of GPT partition attribute bits to set, as defined in
https://uefi.org/specs/UEFI/2.10/05_GUID_Partition_Table_Format.html#defined-gpt-partition-entry-attributes.
Bit 0: "Required Partition", bit 1: "No Block IO Protocol", bit 2: "Legacy BIOS
Bootable". Bits 3-47 are reserved. Bits 48-63 are GUID specific. For example,
ChromeOS Kernel partitions (GUID=fe3a2a5d-4f32-41a7-b725-accc3285a309) use bit
56 for "successful boot" and bits 48-51 for "priority", where 0 means not
bootable, thus bits 56 and 48 need to be set through this property in order to
be able to boot a ChromeOS Kernel partition on a Chromebook, like so:
'partattrs: [56, 48]'.

- fsck -- if set to `false` -- then set fs_passno (man fstab) to 0 meaning no filesystem
checks in boot time. By default is set to `true` allowing checks on boot.

- fsuuid -- file system UUID string. This option is only supported for btrfs,
ext2, ext3, ext4 and xfs.

- partuuid -- GPT partition UUID string.
A version 5 UUID can be easily generated using the uuid5 template function
{{ uuid5 $namespace $data }} $namespace should be a valid UUID and $data can be
any string, to generate reproducible UUID value pass a fixed value of namespace
and data.

- extendedoptions -- list of additional filesystem extended options which need
to be enabled for the partition.

	   # Yaml syntax for mount points:
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

	# Layout example for Raspberry PI 3:
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
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/docker/go-units"
	"github.com/freddierice/go-losetup/v2"
	"github.com/go-debos/fakemachine"
	"github.com/google/uuid"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-debos/debos"
)

/*
Not an exhaustive list, mostly limited by the architectures supported by
debos through QEMU, see the UAPI spec for an exhaustive list:
https://uapi-group.org/specifications/specs/discoverable_partitions_specification/
*/
var GPTPartitionTypeByArch = map[string]map[string]string{
	"arm": {
		"root":            "69dad710-2ce4-4e3c-b16c-21a1d49abed3",
		"root-verity":     "7386cdf2-203c-47a9-a498-f2ecce45a2d6",
		"root-verity-sig": "42b0455f-eb11-491d-98d3-56145ba9d037",
		"usr":             "7d0359a3-02b3-4f0a-865c-654403e70625",
		"usr-verity":      "c215d751-7bcd-4649-be90-6627490a4c05",
		"usr-verity-sig":  "d7ff812f-37d1-4902-a810-d76ba57b975a",
	},

	"arm64": {
		"root":            "b921b045-1df0-41c3-af44-4c6f280d3fae",
		"root-verity":     "df3300ce-d69f-4c92-978c-9bfb0f38d820",
		"root-verity-sig": "6db69de6-29f4-4758-a7a5-962190f00ce3",
		"usr":             "b0e01050-ee5f-4390-949a-9101b17104e9",
		"usr-verity":      "6e11a4e7-fbca-4ded-b9e9-e1a512bb664e",
		"usr-verity-sig":  "c23ce4ff-44bd-4b00-b2d4-b41b3419e02a",
	},

	"i386": {
		"root":            "44479540-f297-41b2-9af7-d131d5f0458a",
		"root-verity":     "d13c5d3b-b5d1-422a-b29f-9454fdc89d76",
		"root-verity-sig": "5996fc05-109c-48de-808b-23fa0830b676",
		"usr":             "75250d76-8cc6-458e-bd66-bd47cc81a812",
		"usr-verity":      "8f461b0d-14ee-4e81-9aa9-049b6fb97abd",
		"usr-verity-sig":  "974a71c0-de41-43c3-be5d-5c5ccd1ad2c0",
	},

	"amd64": {
		"root":            "4f68bce3-e8cd-4db1-96e7-fbcaf984b709",
		"root-verity":     "2c7357ed-ebd2-46d9-aec1-23d437ec2bf5",
		"root-verity-sig": "41092b05-9fc8-4523-994f-2def0408b176",
		"usr":             "8484680c-9521-48c6-9c11-b0720656f69e",
		"usr-verity":      "77ff5f63-e7b6-4633-acf4-1565b864c0e6",
		"usr-verity-sig":  "e7bb33fb-06cf-4e81-8273-e543b413e2e2",
	},

	"mips": {
		"root":            "e9434544-6e2c-47cc-bae2-12d6deafb44c",
		"usr":             "773b2abc-2a99-4398-8bf5-03baac40d02b",
		"root-verity":     "7a430799-f711-4c7e-8e5b-1d685bd48607",
		"usr-verity":      "6e5a1bc8-d223-49b7-bca8-37a5fcceb996",
		"root-verity-sig": "bba210a2-9c5d-45ee-9e87-ff2ccbd002d0",
		"usr-verity-sig":  "97ae158d-f216-497b-8057-f7f905770f54",
	},

	"mipsel": {
		"root":            "37c58c8a-d913-4156-a25f-48b1b64e07f0",
		"usr":             "0f4868e9-9952-4706-979f-3ed3a473e947",
		"root-verity":     "d7d150d2-2a04-4a33-8f12-16651205ff7b",
		"usr-verity":      "46b98d8d-b55c-4e8f-aab3-37fca7f80752",
		"root-verity-sig": "c919cc1f-4456-4eff-918c-f75e94525ca5",
		"usr-verity-sig":  "3e23ca0b-a4bc-4b4e-8087-5ab6a26aa8a9",
	},

	"mips64el": {
		"root":            "700bda43-7a34-4507-b179-eeb93d7a7ca3",
		"usr":             "c97c1f32-ba06-40b4-9f22-236061b08aa8",
		"root-verity":     "16b417f8-3e06-4f57-8dd2-9b5232f41aa6",
		"usr-verity":      "3c3d61fe-b5f3-414d-bb71-8739a694a4ef",
		"root-verity-sig": "904e58ef-5c65-4a31-9c57-6af5fc7c5de7",
		"usr-verity-sig":  "f2c2c7ee-adcc-4351-b5c6-ee9816b66e16",
	},

	"riscv64": {
		"root":            "72ec70a6-cf74-40e6-bd49-4bda08e8f224",
		"usr":             "beaec34b-8442-439b-a40b-984381ed097d",
		"root-verity":     "b6ed5582-440b-4209-b8da-5ff7c419ea3d",
		"usr-verity":      "8f1056be-9b05-47c4-81d6-be53128e5b54",
		"root-verity-sig": "efe0f087-ea8d-4469-821a-4c2a96a8386a",
		"usr-verity-sig":  "d2f9000a-7a18-453f-b5cd-4d32f77a7b32",
	},
}

var GPTPartitionTypeArchless = map[string]string{
	"esp":           "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
	"xbootldr":      "bc13c2ff-59e6-4262-a352-b275fd6f7172",
	"swap":          "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f",
	"home":          "933ac7e1-2eb4-4f13-b844-0e14e2aef915",
	"srv":           "3b8f8425-20e0-4f3b-907f-1a25a76f98e8",
	"var":           "4d21b016-b534-45c2-a9fb-5c16e091fd2d",
	"tmp":           "7ec6f557-3bc5-4aca-b293-16ef5df639d1",
	"user-home":     "773f91ef-66d4-49b5-bd83-d683bf40ad16",
	"linux-generic": "0fc63daf-8483-4772-8e79-3d69d8477de4",
}

func findPartTypeUUID(context *debos.Context, friendlyName string) (string, error) {
	if typeUUID, ok := GPTPartitionTypeArchless[friendlyName]; ok {
		return typeUUID, nil
	}

	arch := context.Architecture
	switch arch {
	case "armhf", "armel":
		arch = "arm"
	}

	submap, ok := GPTPartitionTypeByArch[arch]
	if !ok {
		return "", fmt.Errorf("no GPT type identifiers for arch %s", arch)
	}

	if typeUUID, ok := submap[friendlyName]; ok {
		return typeUUID, nil
	}

	return "", fmt.Errorf("couldn't map %s against any known partition type UUIDs", friendlyName)
}

type Partition struct {
	number          int
	Name            string
	PartLabel       string
	FSLabel         string
	PartType        string
	PartAttrs       []string
	PartUUID        string
	Start           string
	End             string
	FS              string
	Flags           []string
	Features        []string
	ExtendedOptions []string
	Fsck            bool `yaml:"fsck"`
	FSUUID          string
}

type Mountpoint struct {
	Mountpoint string
	Partition  string
	Options    []string
	Buildtime  bool
	part       *Partition
}

type imageLocker struct {
	fd *os.File
}

func lockImage(context *debos.Context) (*imageLocker, error) {
	fd, err := os.Open(context.Image)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX); err != nil {
		fd.Close()
		return nil, fmt.Errorf("failed to lock image: %w", err)
	}
	return &imageLocker{fd: fd}, nil
}

func (i imageLocker) unlock() {
	i.fd.Close()
}

type ImagePartitionAction struct {
	debos.BaseAction `yaml:",inline"`
	ImageName        string
	ImageSize        string
	PartitionType    string
	DiskID           string
	GptGap           string `yaml:"gpt_gap"`
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

func (i *ImagePartitionAction) generateFSTab(context *debos.Context) error {
	context.ImageFSTab.Reset()

	for _, m := range i.Mountpoints {
		options := []string{"defaults"}
		options = append(options, m.Options...)
		if m.Buildtime {
			/* Do not need to add mount point into fstab */
			continue
		}
		if m.part.FSUUID == "" {
			return fmt.Errorf("missing fs UUID for partition %s", m.part.Name)
		}

		fsPassno := 0

		if m.part.Fsck {
			if m.Mountpoint == "/" {
				fsPassno = 1
			} else {
				fsPassno = 2
			}
		}

		fsType := m.part.FS
		switch m.part.FS {
		case "fat", "fat12", "fat16", "fat32", "msdos":
			fsType = "vfat"
		}

		context.ImageFSTab.WriteString(fmt.Sprintf("UUID=%s\t%s\t%s\t%s\t0\t%d\n",
			m.part.FSUUID, m.Mountpoint, fsType,
			strings.Join(options, ","), fsPassno))
	}

	return nil
}

func (i *ImagePartitionAction) generateKernelRoot(context *debos.Context) error {
	for _, m := range i.Mountpoints {
		if m.Mountpoint == "/" {
			if m.part.FSUUID == "" {
				return errors.New("no fs UUID for root partition")
			}
			context.ImageKernelRoot = fmt.Sprintf("root=UUID=%s", m.part.FSUUID)
			break
		}
	}

	return nil
}

func (i ImagePartitionAction) getPartitionDevice(number int, context debos.Context) string {
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
	}
	return fmt.Sprintf("%s%d", device, number)
}

func (i *ImagePartitionAction) triggerDeviceNodes(context *debos.Context) error {
	err := debos.Command{}.Run("udevadm", "udevadm", "trigger", "--settle", context.Image)
	if err != nil {
		log.Printf("Failed to trigger device nodes")
		return err
	}

	return nil
}

func (i ImagePartitionAction) PreMachine(context *debos.Context, m *fakemachine.Machine,
	args *[]string) error {
	imagePath := path.Join(context.Artifactdir, i.ImageName)
	image, err := m.CreateImage(imagePath, i.size)
	if err != nil {
		return err
	}

	context.Image = image
	*args = append(*args, "--internal-image", image)
	return nil
}

func (i ImagePartitionAction) formatPartition(p *Partition, context debos.Context) error {
	label := fmt.Sprintf("Formatting partition %d", p.number)
	path := i.getPartitionDevice(p.number, context)

	cmdline := []string{}
	switch p.FS {
	case "fat", "fat12", "fat16", "fat32", "msdos", "vfat":
		cmdline = append(cmdline, "mkfs.vfat", "-n", p.FSLabel)

		switch p.FS {
		case "fat12":
			cmdline = append(cmdline, "-F12")
		case "fat16":
			cmdline = append(cmdline, "-F16")
		case "fat32", "msdos", "vfat":
			cmdline = append(cmdline, "-F32")
		default:
			/* let mkfs.vfat autodetermine FAT type */
			break
		}

		if len(p.FSUUID) > 0 {
			cmdline = append(cmdline, "-i", p.FSUUID)
		}
	case "btrfs":
		// Force formatting to prevent failure in case if partition was formatted already
		cmdline = append(cmdline, "mkfs.btrfs", "-L", p.FSLabel, "-f")
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
		if len(p.FSUUID) > 0 {
			cmdline = append(cmdline, "-U", p.FSUUID)
		}
	case "f2fs":
		cmdline = append(cmdline, "mkfs.f2fs", "-l", p.FSLabel)
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
	case "hfs":
		cmdline = append(cmdline, "mkfs.hfs", "-h", "-v", p.FSLabel)
	case "hfsplus":
		cmdline = append(cmdline, "mkfs.hfsplus", "-v", p.FSLabel)
	case "hfsx":
		cmdline = append(cmdline, "mkfs.hfsplus", "-s", "-v", p.FSLabel)
		// hfsx is case-insensitive hfs+, should be treated as "normal" hfs+ from now on
		p.FS = "hfsplus"
	case "xfs":
		cmdline = append(cmdline, "mkfs.xfs", "-L", p.FSLabel)
		if len(p.FSUUID) > 0 {
			cmdline = append(cmdline, "-m", "uuid="+p.FSUUID)
		}
	case "none":
	default:
		cmdline = append(cmdline, fmt.Sprintf("mkfs.%s", p.FS), "-L", p.FSLabel)
		if len(p.Features) > 0 {
			cmdline = append(cmdline, "-O", strings.Join(p.Features, ","))
		}
		if len(p.ExtendedOptions) > 0 {
			cmdline = append(cmdline, "-E", strings.Join(p.ExtendedOptions, ","))
		}
		if len(p.FSUUID) > 0 {
			if p.FS == "ext2" || p.FS == "ext3" || p.FS == "ext4" {
				cmdline = append(cmdline, "-U", p.FSUUID)
			}
		}
	}

	if len(cmdline) != 0 {
		cmdline = append(cmdline, path)

		cmd := debos.Command{}

		/* Some underlying device driver, e.g. the UML UBD driver, may manage holes
		 * incorrectly which will prevent to retrieve all useful zero ranges in
		 * filesystem, e.g. when using 'bmaptool create', see patch
		 * http://lists.infradead.org/pipermail/linux-um/2022-January/002074.html
		 *
		 * Adding UNIX_IO_NOZEROOUT environment variable prevent mkfs.ext[234]
		 * utilities to create zero range spaces using fallocate with
		 * FALLOC_FL_ZERO_RANGE or FALLOC_FL_PUNCH_HOLE */
		if p.FS == "ext2" || p.FS == "ext3" || p.FS == "ext4" {
			cmd.AddEnv("UNIX_IO_NOZEROOUT=1")
		}

		if err := cmd.Run(label, cmdline...); err != nil {
			return err
		}
	}

	if p.FS != "none" && p.FSUUID == "" {
		uuid, err := exec.Command("blkid", "-o", "value", "-s", "UUID", "-p", "-c", "none", path).Output()
		if err != nil {
			return fmt.Errorf("failed to get uuid: %w", err)
		}
		p.FSUUID = strings.TrimSpace(string(uuid[:]))
	}

	return nil
}

func (i *ImagePartitionAction) PreNoMachine(context *debos.Context) error {
	imagePath := path.Join(context.Artifactdir, i.ImageName)
	img, err := os.OpenFile(imagePath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("couldn't open image file: %w", err)
	}

	err = img.Truncate(i.size)
	if err != nil {
		return fmt.Errorf("couldn't resize image file: %w", err)
	}

	img.Close()

	// losetup.Attach() can fail due to concurrent attaches in other processes
	retries := 60
	for t := 1; t <= retries; t++ {
		i.loopDev, err = losetup.Attach(imagePath, 0, false)
		if err == nil {
			break
		}
		log.Printf("Setup loop device: try %d/%d failed: %v", t, retries, err)
		time.Sleep(200 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("failed to setup loop device: %w", err)
	}

	// go-losetup doesn't provide a way to change the loop device sector size
	// see https://github.com/freddierice/go-losetup/pull/10
	if context.SectorSize != 512 {
		command := []string{"losetup", "--sector-size", strconv.Itoa(context.SectorSize), i.loopDev.Path()}
		err = debos.Command{}.Run("losetup", command...)
		if err != nil {
			return err
		}
	}

	context.Image = i.loopDev.Path()
	i.usingLoop = true

	return nil
}

func (i ImagePartitionAction) Run(context *debos.Context) error {
	/* On certain disk device events udev will call the BLKRRPART ioctl to
	 * re-read the partition table. This will cause the partition devices
	 * (e.g. vda3) to temporarily disappear while the rescanning happens.
	 * udev does this while holding an exclusive flock. This means to avoid partition
	 * devices disappearing while doing operations on them (e.g. formatting
	 * and mounting) we need to do it while holding an exclusive lock
	 */
	command := []string{"parted", "-s", context.Image, "mklabel", i.PartitionType}
	if len(i.GptGap) > 0 {
		command = append(command, i.GptGap)
	}
	err := debos.Command{}.Run("parted", command...)
	if err != nil {
		return err
	}

	if len(i.DiskID) > 0 {
		command := []string{"sfdisk", "--disk-id", context.Image, i.DiskID}
		err = debos.Command{}.Run("sfdisk", command...)
		if err != nil {
			return err
		}
	}

	for idx := range i.Partitions {
		p := &i.Partitions[idx]

		if p.PartLabel == "" {
			p.PartLabel = p.Name
		}

		var name string
		if i.PartitionType == "msdos" {
			if len(i.Partitions) <= 4 {
				name = "primary"
			} else {
				if idx < 3 {
					name = "primary"
				} else if idx == 3 {
					name = "extended"
				} else {
					name = "logical"
				}
			}
		} else {
			name = p.PartLabel
		}

		command := []string{"parted", "-a", "none", "-s", "--", context.Image, "mkpart", name}
		switch p.FS {
		case "fat16":
			command = append(command, "fat16")
		case "fat", "fat12", "fat32", "msdos", "vfat":
			/* TODO: Not sure if this is correct. Perhaps
			   fat12 should be treated the same as fat16 ? */
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

		if len(p.PartAttrs) > 0 {
			/* Convert bits numbers to bits names due to a libfdisk's limitation
			 * https://github.com/util-linux/util-linux/issues/3353
			 */
			for idx, attr := range p.PartAttrs {
				switch attr {
				case "0":
					p.PartAttrs[idx] = "RequiredPartition"
				case "1":
					p.PartAttrs[idx] = "NoBlockIOProtocol"
				case "2":
					p.PartAttrs[idx] = "LegacyBIOSBootable"
				}
			}
			err = debos.Command{}.Run("sfdisk", "sfdisk", "--part-attrs", context.Image, fmt.Sprintf("%d", p.number), strings.Join(p.PartAttrs, ","))
			if err != nil {
				return err
			}
		}

		/* PartUUID will only be set for gpt partitions */
		if len(p.PartUUID) > 0 {
			err = debos.Command{}.Run("sfdisk", "sfdisk", "--part-uuid", context.Image, fmt.Sprintf("%d", p.number), p.PartUUID)
			if err != nil {
				return err
			}
		}

		lock, err := lockImage(context)
		if err != nil {
			return err
		}
		defer lock.unlock()

		err = i.formatPartition(p, *context)
		if err != nil {
			return err
		}
		lock.unlock()

		devicePath := i.getPartitionDevice(p.number, *context)
		context.ImagePartitions = append(context.ImagePartitions,
			debos.Partition{Name: p.Name, DevicePath: devicePath})
	}

	context.ImageMntDir = path.Join(context.Scratchdir, "mnt")
	if err := os.MkdirAll(context.ImageMntDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	// sort mountpoints based on position in filesystem hierarchy
	sort.SliceStable(i.Mountpoints, func(a, b int) bool {
		mntA := i.Mountpoints[a].Mountpoint
		mntB := i.Mountpoints[b].Mountpoint

		// root should always be mounted first
		if mntA == "/" {
			return true
		}
		if mntB == "/" {
			return false
		}

		return strings.Count(mntA, "/") < strings.Count(mntB, "/")
	})

	lock, err := lockImage(context)
	if err != nil {
		return err
	}
	defer lock.unlock()

	for _, m := range i.Mountpoints {
		dev := i.getPartitionDevice(m.part.number, *context)
		mntpath := path.Join(context.ImageMntDir, m.Mountpoint)
		if err := os.MkdirAll(mntpath, 0755); err != nil {
			return fmt.Errorf("failed to create mountpoint %s: %w", mntpath, err)
		}
		fsType := m.part.FS
		switch m.part.FS {
		case "fat", "fat12", "fat16", "fat32", "msdos":
			fsType = "vfat"
		}
		err = syscall.Mount(dev, mntpath, fsType, 0, "")
		if err != nil {
			return fmt.Errorf("%s mount failed: %w", m.part.Name, err)
		}
	}
	lock.unlock()

	err = i.generateFSTab(context)
	if err != nil {
		return err
	}

	err = i.generateKernelRoot(context)
	if err != nil {
		return err
	}

	/* Now that all partitions are created (re)trigger all udev events for
	 * the image file to make sure everything is in a reasonable state
	 */
	if err := i.triggerDeviceNodes(context); err != nil {
		return fmt.Errorf("failed to trigger device nodes: %w", err)
	}
	return nil
}

func (i ImagePartitionAction) Cleanup(context *debos.Context) error {
	for idx := len(i.Mountpoints) - 1; idx >= 0; idx-- {
		m := i.Mountpoints[idx]
		mntpath := path.Join(context.ImageMntDir, m.Mountpoint)
		err := syscall.Unmount(mntpath, 0)
		if err != nil {
			log.Printf("Warning: Failed to get unmount %s: %s", m.Mountpoint, err)
			log.Printf("Unmount failure can cause images being incomplete!")
			return err
		}
		if m.Buildtime {
			if err = os.Remove(mntpath); err != nil {
				log.Printf("Failed to remove temporary mount point %s: %s", m.Mountpoint, err)

				var pathErr *os.PathError
				if errors.As(err, &pathErr) && pathErr.Err.Error() == "read-only file system" {
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
			time.Sleep(time.Second)
		}

		if err != nil {
			log.Printf("WARNING: Failed to remove loop device: %s", err)
			return err
		}
	}

	return nil
}

func (i ImagePartitionAction) PostMachineCleanup(context *debos.Context) error {
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

func (i *ImagePartitionAction) Verify(context *debos.Context) error {
	if i.PartitionType == "msdos" {
		for idx := range i.Partitions {
			p := &i.Partitions[idx]

			if idx == 3 && len(i.Partitions) > 4 {
				var name string
				var part Partition

				name = "extended"
				part.number = idx + 1
				part.Name = name
				part.Start = p.Start
				tmpN := len(i.Partitions) - 1
				tmp := &i.Partitions[tmpN]
				part.End = tmp.End
				part.FS = "none"

				i.Partitions = append(i.Partitions[:idx+1], i.Partitions[idx:]...)
				i.Partitions[idx] = part

				num := 1
				for idx := range i.Partitions {
					p := &i.Partitions[idx]
					p.number = num
					num++
				}
			}
		}
	}

	if len(i.GptGap) > 0 {
		log.Println("WARNING: special version of parted is needed for 'gpt_gap' option")
		if i.PartitionType != "gpt" {
			return fmt.Errorf("gpt_gap property could be used only with 'gpt' label")
		}
		// Just check if it contains correct value
		_, err := units.FromHumanSize(i.GptGap)
		if err != nil {
			return fmt.Errorf("failed to parse image size: %s", i.GptGap)
		}
	}

	if len(i.DiskID) > 0 {
		switch i.PartitionType {
		case "gpt":
			_, err := uuid.Parse(i.DiskID)
			if err != nil {
				return fmt.Errorf("incorrect disk GUID %s", i.DiskID)
			}
		case "msdos":
			_, err := hex.DecodeString(i.DiskID)
			if err != nil || len(i.DiskID) != 8 {
				return fmt.Errorf("incorrect disk ID %s, should be 32-bit hexadecimal number", i.DiskID)
			}
			// Add 0x prefix
			i.DiskID = "0x" + i.DiskID
		}
	}

	num := 1
	for idx := range i.Partitions {
		var maxLength = 0
		p := &i.Partitions[idx]
		p.number = num
		num++
		if p.Name == "" {
			return fmt.Errorf("partition without a name")
		}

		// check for duplicate partition names
		for j := idx + 1; j < len(i.Partitions); j++ {
			if i.Partitions[j].Name == p.Name {
				return fmt.Errorf("partition %s already exists", p.Name)
			}
		}

		if len(p.FSUUID) > 0 {
			switch p.FS {
			case "btrfs", "ext2", "ext3", "ext4", "xfs":
				_, err := uuid.Parse(p.FSUUID)
				if err != nil {
					return fmt.Errorf("incorrect UUID %s", p.FSUUID)
				}
			case "fat", "fat12", "fat16", "fat32", "msdos", "vfat":
				_, err := hex.DecodeString(p.FSUUID)
				if err != nil || len(p.FSUUID) != 8 {
					return fmt.Errorf("incorrect UUID %s, should be 32-bit hexadecimal number", p.FSUUID)
				}
			default:
				return fmt.Errorf("setting the UUID is not supported for filesystem %s", p.FS)
			}
		}

		if i.PartitionType != "gpt" && p.PartLabel != "" {
			return fmt.Errorf("can only set partition partlabel on GPT filesystem")
		}

		if len(p.PartUUID) > 0 {
			switch i.PartitionType {
			case "gpt":
				_, err := uuid.Parse(p.PartUUID)
				if err != nil {
					return fmt.Errorf("incorrect partition UUID %s", p.PartUUID)
				}
			default:
				return fmt.Errorf("setting the partition UUID is not supported for %s", i.PartitionType)
			}
		}

		if len(p.PartType) > 0 {
			if i.PartitionType == "gpt" {
				_, err := uuid.Parse(p.PartType)
				if err != nil { // Not a valid UUID, but might be a valid short name
					typeUUID, err := findPartTypeUUID(context, p.PartType)
					if err != nil {
						return fmt.Errorf("GPT partition type '%s' for partition '%s' is invalid UUID and not a recognized identifier", p.PartType, p.Name)
					}

					p.PartType = typeUUID
				}
			} else {
				partTypeLen := 2 // msdos
				if len(p.PartType) != partTypeLen {
					return fmt.Errorf("incorrect partition type for '%s', should be %d characters", p.Name, partTypeLen)
				}
			}
		}

		for _, bitStr := range p.PartAttrs {
			bit, err := strconv.ParseInt(bitStr, 0, 0)
			if err != nil || bit < 0 || bit > 2 && bit < 48 || bit > 63 {
				return fmt.Errorf("partition attribute bit '%s' outside of valid range (0-2, 48-63)", bitStr)
			}
		}

		if p.Start == "" {
			return fmt.Errorf("partition %s missing start", p.Name)
		}
		if p.End == "" {
			return fmt.Errorf("partition %s missing end", p.Name)
		}

		if p.FS == "" {
			return fmt.Errorf("partition %s missing fs type", p.Name)
		}

		if p.FSLabel == "" {
			p.FSLabel = p.Name
		}

		switch p.FS {
		case "fat", "fat12", "fat16", "fat32", "msdos", "vfat":
			maxLength = 11
		case "ext2", "ext3", "ext4":
			maxLength = 16
		case "btrfs":
			maxLength = 255
		case "f2fs":
			maxLength = 512
		case "hfs", "hfsplus":
			maxLength = 255
		case "xfs":
			maxLength = 12
		case "none":
		default:
			log.Printf("Warning: setting a fs label for %s is unsupported", p.FS)
		}

		if maxLength > 0 && len(p.FSLabel) > maxLength {
			return fmt.Errorf("fs label for %s '%s' is too long", p.Name, p.FSLabel)
		}
	}

	for idx := range i.Mountpoints {
		m := &i.Mountpoints[idx]

		if len(m.Mountpoint) == 0 {
			return errors.New("mountpoint property is mandatory for mountpoints")
		}

		if len(m.Partition) == 0 {
			return errors.New("partition property is mandatory for mountpoints")
		}

		// check for duplicate mountpoints
		for j := idx + 1; j < len(i.Mountpoints); j++ {
			if i.Mountpoints[j].Mountpoint == m.Mountpoint {
				return fmt.Errorf("mountpoint %s already exists", m.Mountpoint)
			}
		}

		for pidx := range i.Partitions {
			p := &i.Partitions[pidx]
			if m.Partition == p.Name {
				m.part = p
				break
			}
		}
		if m.part == nil {
			return fmt.Errorf("couldn't find partition for %s", m.Mountpoint)
		}

		if strings.ToLower(m.part.FS) == "none" {
			return fmt.Errorf("cannot mount %s: filesystem not present", m.Mountpoint)
		}
	}

	// Calculate the size based on the unit (binary or decimal)
	// binary units are multiples of 1024 - KiB, MiB, GiB, TiB, PiB
	// decimal units are multiples of 1000 - KB, MB, GB, TB, PB
	var getSizeValueFunc func(size string) (int64, error)
	if regexp.MustCompile(`^[0-9.]+[kmgtp]ib+$`).MatchString(strings.ToLower(i.ImageSize)) {
		getSizeValueFunc = units.RAMInBytes
	} else {
		getSizeValueFunc = units.FromHumanSize
	}

	size, err := getSizeValueFunc(i.ImageSize)
	if err != nil {
		return fmt.Errorf("failed to parse image size: %s", i.ImageSize)
	}

	i.size = size
	return nil
}
