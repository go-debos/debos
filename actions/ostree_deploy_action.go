/*
OstreeDeploy Action

Deploy the OSTree branch to the image.
If any preparation has been done for rootfs, it can be overwritten
during this step.

Action 'image-partition' must be called prior to OSTree deploy.

	# Yaml syntax:
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
	RemoteRepository    string `yaml:"remote_repository"`
	Branch              string
	Os                  string
	SetupFSTab          bool   `yaml:"setup-fstab"`
	SetupKernelCmdline  bool   `yaml:"setup-kernel-cmdline"`
	AppendKernelCmdline string `yaml:"append-kernel-cmdline"`
	TLSClientCertPath   string `yaml:"tls-client-cert-path"`
	TLSClientKeyPath    string `yaml:"tls-client-key-path"`
	CollectionID        string `yaml:"collection-id"`
}

func NewOstreeDeployAction() *OstreeDeployAction {
	ot := &OstreeDeployAction{SetupFSTab: true, SetupKernelCmdline: true}
	ot.Description = "Deploying from ostree"
	return ot
}

func (ot *OstreeDeployAction) setupFSTab(deployment *ostree.Deployment, context *debos.Context) error {
	deploymentDir := fmt.Sprintf("ostree/deploy/%s/deploy/%s.%d",
		deployment.Osname(), deployment.Csum(), deployment.Deployserial())

	etcDir := path.Join(context.Rootdir, deploymentDir, "etc")

	if err := os.Mkdir(etcDir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mkdir %s: %w", etcDir, err)
	}

	dst, err := os.OpenFile(path.Join(etcDir, "fstab"), os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("open fstab for write: %w", err)
	}
	defer dst.Close()

	if _, err = io.Copy(dst, &context.ImageFSTab); err != nil {
		return fmt.Errorf("copy fstab content: %w", err)
	}

	return nil
}

func (ot *OstreeDeployAction) Run(context *debos.Context) error {
	// This is to handle cases there we didn't partition an image
	if len(context.ImageMntDir) != 0 {
		/* First deploy the current rootdir to the image so it can seed e.g.
		 * bootloader configuration */
		err := debos.Command{}.Run("Deploy to image", "cp", "-a", context.Rootdir+"/.", context.ImageMntDir)
		if err != nil {
			return fmt.Errorf("rootfs deploy failed: %w", err)
		}
		context.Rootdir = context.ImageMntDir
		context.Origins["filesystem"] = context.ImageMntDir
	}

	repoPath := "file://" + path.Join(context.Artifactdir, ot.Repository)

	sysroot := ostree.NewSysroot(context.Rootdir)
	err := sysroot.InitializeFS()
	if err != nil {
		return fmt.Errorf("initialize sysroot: %w", err)
	}

	err = sysroot.InitOsname(ot.Os, nil)
	if err != nil {
		return fmt.Errorf("init osname %s: %w", ot.Os, err)
	}

	/* HACK: Getting the repository form the sysroot gets ostree confused on
	 * whether it should configure /etc/ostree or the repo configuration,
	   so reopen by hand */
	/* dstRepo, err := sysroot.Repo(nil) */
	dstRepo, err := ostree.OpenRepo(path.Join(context.Rootdir, "ostree/repo"))
	if err != nil {
		return fmt.Errorf("open ostree repo: %w", err)
	}

	/* FIXME: add support for gpg signing commits so this is no longer needed, see #661 */
	opts := ostree.RemoteOptions{NoGpgVerify: true,
		TlsClientCertPath: ot.TLSClientCertPath,
		TlsClientKeyPath:  ot.TLSClientKeyPath,
		CollectionId:      ot.CollectionID,
	}

	err = dstRepo.RemoteAdd("origin", ot.RemoteRepository, opts, nil)
	if err != nil {
		return fmt.Errorf("remote add: %w", err)
	}

	var options ostree.PullOptions
	options.OverrideRemoteName = "origin"
	options.Refs = []string{ot.Branch}

	err = dstRepo.PullWithOptions(repoPath, options, nil, nil)
	if err != nil {
		return fmt.Errorf("pull from remote %s: %w", repoPath, err)
	}

	/* Required by ostree to make sure a bunch of information was pulled in  */
	if err := sysroot.Load(nil); err != nil {
		return fmt.Errorf("failed to load sysroot: %w", err)
	}

	revision, err := dstRepo.ResolveRev(ot.Branch, false)
	if err != nil {
		return fmt.Errorf("resolve revision %s: %w", ot.Branch, err)
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
		return fmt.Errorf("deploy tree: %w", err)
	}

	if ot.SetupFSTab {
		err = ot.setupFSTab(deployment, context)
		if err != nil {
			return fmt.Errorf("setup fstab: %w", err)
		}
	}

	err = sysroot.SimpleWriteDeployment(ot.Os, deployment, nil, 0, nil)
	if err != nil {
		return fmt.Errorf("write deployment: %w", err)
	}

	/* libostree keeps some information, like repo lock file descriptor, in
	 * thread specific variables. As GC can be run from another thread, it
	 * may not been able to access this, preventing to free them correctly.
	 * To prevent this, explicitly dereference libostree objects. */
	dstRepo.Unref()
	sysroot.Unref()
	return nil
}
