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
	debos.BaseAction          `yaml:",inline"`
	Repository          string
	RemoteRepository    string "remote_repository"
	Branch              string
	Os                  string
	SetupFSTab          bool `yaml:setup-fstab`
	SetupKernelCmdline  bool `yaml:setup-kernel-cmdline`
	AppendKernelCmdline string
}

func NewOstreeDeployAction() *OstreeDeployAction {
	ot := &OstreeDeployAction{SetupFSTab: true, SetupKernelCmdline: true}
	ot.Description = "Deploying from ostree"
	return ot
}

func (ot *OstreeDeployAction) setupFSTab(deployment *ostree.Deployment, context *debos.DebosContext) error {
	deploymentDir := fmt.Sprintf("ostree/deploy/%s/deploy/%s.%d",
		deployment.Osname(), deployment.Csum(), deployment.Deployserial())

	etcDir := path.Join(context.ImageMntDir, deploymentDir, "etc")

	err := os.Mkdir(etcDir, 755)
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
	/* First deploy the current rootdir to the image so it can seed e.g.
	 * bootloader configuration */
	err := debos.Command{}.Run("Deploy to image", "cp", "-a", context.Rootdir+"/.", context.ImageMntDir)
	if err != nil {
		return fmt.Errorf("rootfs deploy failed: %v", err)
	}
	context.Rootdir = context.ImageMntDir

	repoPath := "file://" + path.Join(context.Artifactdir, ot.Repository)

	sysroot := ostree.NewSysroot(context.ImageMntDir)
	err = sysroot.InitializeFS()
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
	dstRepo, err := ostree.OpenRepo(path.Join(context.ImageMntDir, "ostree/repo"))
	if err != nil {
		return err
	}

	/* FIXME: add support for gpg signing commits so this is no longer needed */
	opts := ostree.RemoteOptions{NoGpgVerify: true}
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

	return nil
}
