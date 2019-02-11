/*
FilesystemDeploy Action

Deploy prepared root filesystem to output image. This action requires
'image-partition' action to be executed before it.

Yaml syntax:
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
	"io/ioutil"
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

func (fd *FilesystemDeployAction) setupFSTab(context *debos.DebosContext) error {
	if context.ImageFSTab.Len() == 0 {
		return errors.New("Fstab not generated, missing image-partition action?")
	}

	log.Print("Setting up fstab")

	err := os.MkdirAll(path.Join(context.Rootdir, "etc"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc in image: %v", err)
	}

	fstab := path.Join(context.Rootdir, "etc/fstab")
	f, err := os.OpenFile(fstab, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		return fmt.Errorf("Couldn't open fstab: %v", err)
	}

	_, err = io.Copy(f, &context.ImageFSTab)

	if err != nil {
		return fmt.Errorf("Couldn't write fstab: %v", err)
	}
	f.Close()

	return nil
}

func (fd *FilesystemDeployAction) setupKernelCmdline(context *debos.DebosContext) error {
	var cmdline []string

	log.Print("Setting up /etc/kernel/cmdline")

	err := os.MkdirAll(path.Join(context.Rootdir, "etc", "kernel"), 0755)
	if err != nil {
		return fmt.Errorf("Couldn't create etc/kernel in image: %v", err)
	}
	path := path.Join(context.Rootdir, "etc/kernel/cmdline")
	current, _ := ioutil.ReadFile(path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		log.Fatalf("Couldn't open kernel cmdline: %v", err)
	}

	cmdline = append(cmdline, strings.TrimSpace(string(current)))
	cmdline = append(cmdline, context.ImageKernelRoot)

	if fd.AppendKernelCmdline != "" {
		cmdline = append(cmdline, fd.AppendKernelCmdline)
	}

	_, err = f.WriteString(strings.Join(cmdline, " ") + "\n")
	if err != nil {
		return fmt.Errorf("Couldn't write kernel/cmdline: %v", err)
	}

	f.Close()
	return nil
}

func (fd *FilesystemDeployAction) Run(context *debos.DebosContext) error {
	fd.LogStart()
	/* Copying files is actually silly hafd, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	err := debos.Command{}.Run("Deploy to image", "cp", "-a", context.Rootdir+"/.", context.ImageMntDir)
	if err != nil {
		return fmt.Errorf("rootfs deploy failed: %v", err)
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
