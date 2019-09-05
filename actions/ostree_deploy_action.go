/*
OstreeDeploy Action

Deploy the OSTree branch to the image.
If any preparation has been done for rootfs, it can be overwritten
during this step.

Action 'image-partition' must be called prior to OSTree deploy.

Yaml syntax:
 - action: ostree-deploy
   repository: repository name
   remote_repository: URL
   branch: branch name
   os: os name
   tls-client-cert-path: path to client certificate
   tls-client-key-path: path to client certificate key
   setup-fstab: bool
   setup-kernel-cmdline: bool
   appendkernelcmdline: arguments
   collection-id: org.apertis.example

Mandatory properties:

- remote_repository -- URL to remote OSTree repository for pulling stateroot branch.
Currently not implemented, please prepare local repository instead.

- repository -- path to repository with OSTree structure.
This path is relative to 'artifact' directory.

- os -- os deployment name, as explained in:
https://ostree.readthedocs.io/en/latest/manual/deployment/

- branch -- branch of the repository to use for populating the image.

Optional properties:

- setup-fstab -- create '/etc/fstab' file for image

- setup-kernel-cmdline -- add the information from the 'image-partition'
action to the configured commandline.

- append-kernel-cmdline -- additional kernel command line arguments passed to kernel.

- tls-client-cert-path -- path to client certificate to use for the remote repository

- tls-client-key-path -- path to client certificate key to use for the remote repository

- collection-id -- Collection ID ref binding (require libostree 2018.6).
*/
package actions

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-debos/debos"
	ostree "github.com/sjoerdsimons/ostree-go/pkg/otbuiltin"
)

type OstreeDeployAction struct {
	debos.BaseAction    `yaml:",inline"`
	Repository          string
	RemoteRepository    string "remote_repository"
	Branch              string
	Os                  string
	SetupFSTab          bool   `yaml:"setup-fstab"`
	SetupKernelCmdline  bool   `yaml:"setup-kernel-cmdline"`
	AppendKernelCmdline string `yaml:"append-kernel-cmdline"`
	TlsClientCertPath   string `yaml:"tls-client-cert-path"`
	TlsClientKeyPath    string `yaml:"tls-client-key-path"`
	CollectionID        string `yaml:"collection-id"`
}

func NewOstreeDeployAction() *OstreeDeployAction {
	ot := &OstreeDeployAction{SetupFSTab: true, SetupKernelCmdline: true}
	ot.Description = "Deploying from ostree"
	return ot
}

func (ot *OstreeDeployAction) setupFSTab(deployment *ostree.Deployment, context *debos.DebosContext) error {
	deploymentDir := fmt.Sprintf("ostree/deploy/%s/deploy/%s.%d",
		deployment.Osname(), deployment.Csum(), deployment.Deployserial())

	etcDir := path.Join(context.Rootdir, deploymentDir, "etc")

	err := os.Mkdir(etcDir, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	dst, err := os.OpenFile(path.Join(etcDir, "fstab"), os.O_WRONLY|os.O_CREATE, 0755)
	defer dst.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, &context.ImageFSTab)

	return err
}

func (ot *OstreeDeployAction) Run(context *debos.DebosContext) error {
	ot.LogStart()

	// This is to handle cases there we didn't partition an image
	if len(context.ImageMntDir) != 0 {
		/* First deploy the current rootdir to the image so it can seed e.g.
		 * bootloader configuration */
		err := debos.Command{}.Run("Deploy to image", "cp", "-a", context.Rootdir+"/.", context.ImageMntDir)
		if err != nil {
			return fmt.Errorf("rootfs deploy failed: %v", err)
		}
		context.Rootdir = context.ImageMntDir
		context.Origins["filesystem"] = context.ImageMntDir
	}

	repoPath := "file://" + path.Join(context.Artifactdir, ot.Repository)

	sysroot := ostree.NewSysroot(context.Rootdir)
	err := sysroot.InitializeFS()
	if err != nil {
		return err
	}

	err = sysroot.InitOsname(ot.Os, nil)
	if err != nil {
		return err
	}

	/* HACK: Getting the repository form the sysroot gets ostree confused on
	 * whether it should configure /etc/ostree or the repo configuration,
	   so reopen by hand */
	/* dstRepo, err := sysroot.Repo(nil) */
	dstRepo, err := ostree.OpenRepo(path.Join(context.Rootdir, "ostree/repo"))
	if err != nil {
		return err
	}

	/* FIXME: add support for gpg signing commits so this is no longer needed */
	opts := ostree.RemoteOptions{NoGpgVerify: true,
		TlsClientCertPath: ot.TlsClientCertPath,
		TlsClientKeyPath:  ot.TlsClientKeyPath,
		CollectionId:      ot.CollectionID,
	}

	err = dstRepo.RemoteAdd("origin", ot.RemoteRepository, opts, nil)
	if err != nil {
		return err
	}

	var options ostree.PullOptions
	options.OverrideRemoteName = "origin"
	options.Refs = []string{ot.Branch}

	err = dstRepo.PullWithOptions(repoPath, options, nil, nil)
	if err != nil {
		return err
	}

	/* Required by ostree to make sure a bunch of information was pulled in  */
	sysroot.Load(nil)

	revision, err := dstRepo.ResolveRev(ot.Branch, false)
	if err != nil {
		return err
	}

	var kargs []string
	if ot.SetupKernelCmdline {
		kargs = append(kargs, context.ImageKernelRoot)
	}

	if ot.AppendKernelCmdline != "" {
		s := strings.Split(ot.AppendKernelCmdline, " ")
		kargs = append(kargs, s...)
	}

	origin := sysroot.OriginNewFromRefspec("origin:" + ot.Branch)
	deployment, err := sysroot.DeployTree(ot.Os, revision, origin, nil, kargs, nil)
	if err != nil {
		return err
	}

	if ot.SetupFSTab {
		err = ot.setupFSTab(deployment, context)
		if err != nil {
			return err
		}
	}

	err = sysroot.SimpleWriteDeployment(ot.Os, deployment, nil, 0, nil)
	if err != nil {
		return err
	}

	/* libostree keeps some information, like repo lock file descriptor, in
	 * thread specific variables. As GC can be run from another thread, it
	 * may not been able to access this, preventing to free them correctly.
	 * To prevent this, explicitly dereference libostree objects. */
	dstRepo.Unref()
	sysroot.Unref()
	return nil
}
