/*
FilesystemDeploy Action

Deploy prepared root filesystem to output image by copying the files from the
temporary scratch directory to the mounted image and optionally creates various
configuration files for the image: '/etc/fstab' and '/etc/kernel/cmdline'. This
action requires 'image-partition' action to be executed before it.

After this action has ran, subsequent actions are executed on the mounted output
image.

	# Yaml syntax:
	- action: filesystem-deploy
	  setup-fstab: bool
	  setup-kernel-cmdline: bool
	  append-kernel-cmdline: arguments

Optional properties:

- setup-fstab -- generate '/etc/fstab' file according to information provided
by 'image-partition' action. By default is 'true'.

- setup-kernel-cmdline -- add location of root partition to '/etc/kernel/cmdline'
file on target image. By default is 'true'.

- append-kernel-cmdline -- additional kernel command line arguments passed to kernel.
*/
package actions

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

type FilesystemDeployAction struct {
	debos.BaseAction    `yaml:",inline"`
	SetupFSTab          bool   `yaml:"setup-fstab"`
	SetupKernelCmdline  bool   `yaml:"setup-kernel-cmdline"`
	AppendKernelCmdline string `yaml:"append-kernel-cmdline"`
}

func NewFilesystemDeployAction() *FilesystemDeployAction {
	fd := &FilesystemDeployAction{SetupFSTab: true, SetupKernelCmdline: true}
	fd.Description = "Deploying filesystem"

	return fd
}

func (fd *FilesystemDeployAction) CheckEnvironment(_ *debos.Context) error {
	cmd := debos.Command{}
	if err := cmd.CheckExecutableExists("cp"); err != nil {
		return err
	}
	return nil
}

func (fd *FilesystemDeployAction) setupFSTab(context *debos.Context) error {
	if context.ImageFSTab.Len() == 0 {
		return errors.New("fstab not generated, missing image-partition action?")
	}

	log.Print("Setting up /etc/fstab")

	err := os.MkdirAll(path.Join(context.Rootdir, "etc"), 0755)
	if err != nil {
		return fmt.Errorf("couldn't create etc in image: %w", err)
	}

	fstab := path.Join(context.Rootdir, "etc/fstab")
	f, err := os.OpenFile(fstab, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("couldn't open /etc/fstab: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, &context.ImageFSTab)

	if err != nil {
		return fmt.Errorf("couldn't write /etc/fstab: %w", err)
	}

	return nil
}

func (fd *FilesystemDeployAction) setupKernelCmdline(context *debos.Context) error {
	var cmdline []string

	log.Print("Setting up /etc/kernel/cmdline")

	err := os.MkdirAll(path.Join(context.Rootdir, "etc", "kernel"), 0755)
	if err != nil {
		return fmt.Errorf("couldn't create etc/kernel in image: %w", err)
	}
	path := path.Join(context.Rootdir, "etc/kernel/cmdline")
	current, _ := os.ReadFile(path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("couldn't open /etc/kernel/cmdline: %w", err)
	}
	defer f.Close()

	cmdline = append(cmdline, strings.TrimSpace(string(current)))
	cmdline = append(cmdline, context.ImageKernelRoot)

	if fd.AppendKernelCmdline != "" {
		cmdline = append(cmdline, fd.AppendKernelCmdline)
	}

	_, err = f.WriteString(strings.Join(cmdline, " ") + "\n")
	if err != nil {
		return fmt.Errorf("couldn't write /etc/kernel/cmdline: %w", err)
	}

	return nil
}

func (fd *FilesystemDeployAction) Run(context *debos.Context) error {
	/* Copying files is actually silly hafd, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	err := debos.Command{}.Run("Deploy to image", "cp", "-a", context.Rootdir+"/.", context.ImageMntDir)
	if err != nil {
		return fmt.Errorf("rootfs deploy failed: %w", err)
	}
	context.Rootdir = context.ImageMntDir
	context.Origins["filesystem"] = context.ImageMntDir

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
