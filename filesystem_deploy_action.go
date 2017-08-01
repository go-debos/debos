package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

type FilesystemDeployAction struct {
	BaseAction         `yaml:",inline"`
	SetupFSTab         bool `yaml:setup-fstab`
	SetupKernelCmdline bool `yaml:setup-kernel-cmdline`
}

func newFilesystemDeployAction() *FilesystemDeployAction {
	fd := &FilesystemDeployAction{SetupFSTab: true, SetupKernelCmdline: true}
	fd.Description = "Deploying filesystem"

	return fd
}

func (fd *FilesystemDeployAction) setupFSTab(context *YaibContext) error {
	if context.imageFSTab.Len() == 0 {
		return errors.New("Fstab not generated, missing image-partition action?")
	}

	log.Print("Setting up fstab")

	err := os.MkdirAll(path.Join(context.rootdir, "etc"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc in image: %v", err)
	}

	fstab := path.Join(context.rootdir, "etc/fstab")
	f, err := os.OpenFile(fstab, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		return fmt.Errorf("Couldn't open fstab: %v", err)
	}

	_, err = io.Copy(f, &context.imageFSTab)

	if err != nil {
		return fmt.Errorf("Couldn't write fstab: %v", err)
	}
	f.Close()

	return nil
}

func (fd *FilesystemDeployAction) setupKernelCmdline(context *YaibContext) error {
	log.Print("Setting up /etc/kernel/cmdline")

	err := os.MkdirAll(path.Join(context.rootdir, "etc", "kernel"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc/kernel in image: %v", err)
	}
	path := path.Join(context.rootdir, "etc/kernel/cmdline")
	current, _ := ioutil.ReadFile(path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		log.Fatalf("Couldn't open kernel cmdline: %v", err)
	}

	cmdline := fmt.Sprintf("%s %s\n",
		strings.TrimSpace(string(current)),
		context.imageKernelRoot)

	_, err = f.WriteString(cmdline)
	if err != nil {
		return fmt.Errorf("Couldn't write kernel/cmdline: %v", err)
	}

	f.Close()
	return nil
}

func (fd *FilesystemDeployAction) Run(context *YaibContext) error {
	fd.LogStart()
	/* Copying files is actually silly hafd, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	err := Command{}.Run("Deploy to image", "cp", "-a", context.rootdir+"/.", context.imageMntDir)
	if err != nil {
		return fmt.Errorf("rootfs deploy failed: %v", err)
	}
	context.rootdir = context.imageMntDir

	if fd.SetupFSTab {
		err = fd.setupFSTab(context)
		if err != nil {
			return err
		}
	}
	if fd.SetupKernelCmdline {
		err = fd.setupKernelCmdline(context)
		if err != nil {
			return err
		}
	}

	return nil
}
