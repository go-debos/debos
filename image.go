package debos

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func CreateImage(imagepath string, size int64) error {
	img, err := os.OpenFile(imagepath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Couldn't open image file: %v", err)
	}

	err = img.Truncate(size)
	if err != nil {
		return fmt.Errorf("Couldn't resize image file: %v", err)
	}

	img.Close()

	return nil
}

func Format(cmdlabel string, imagepath string, fs string, label string) error {
	cmdline := []string{}

	switch fs {
	case "vfat":
		cmdline = append(cmdline, "mkfs.vfat", "-n", label)
	case "btrfs":
		// Force formatting to prevent failure in case partition was formatted already
		cmdline = append(cmdline, "mkfs.btrfs", "-L", label, "-f")
	case "none":
	default:
		cmdline = append(cmdline, fmt.Sprintf("mkfs.%s", fs), "-L", label)
	}

	if len(cmdline) != 0 {
		cmdline = append(cmdline, imagepath)

		cmd := Command{}
		if err := cmd.Run(cmdlabel, cmdline...); err != nil {
			return err
		}
	}

	return nil
}

func SetupLoopDevice(imagepath string) (string, error) {
	loop, err := exec.Command("losetup", "-f", "--show", imagepath).Output()
	if err != nil {
		return "", fmt.Errorf("Failed to setup loop device: %v", err)
	}

	return strings.TrimSpace(string(loop[:])), nil
}

func DetachLoopDevice(imagepath string) error {
	err := exec.Command("losetup", "-d", imagepath).Run()
	if err != nil {
		return fmt.Errorf("Failed to detach loop device: %v", err)
	}

	return nil
}
