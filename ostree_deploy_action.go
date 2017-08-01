package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	ostree "github.com/sjoerdsimons/ostree-go/pkg/otbuiltin"
)

type OstreeDeployAction struct {
	*BaseAction
	Repository       string
	RemoteRepository string "remote_repository"
	Branch           string
	Os               string
}

func (ot *OstreeDeployAction) Run(context *YaibContext) error {
	repoPath := "file://" + path.Join(context.artifactdir, ot.Repository)

	sysroot := ostree.NewSysroot(context.imageMntDir)
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
	dstRepo, err := ostree.OpenRepo(path.Join(context.imageMntDir, "ostree/repo"))
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

	cmdline, _ := ioutil.ReadFile(path.Join(context.imageMntDir, "etc/kernel/cmdline"))
	kargs := strings.Split(strings.TrimSpace(string(cmdline)), " ")

	origin := sysroot.OriginNewFromRefspec("origin:" + ot.Branch)
	deployment, err := sysroot.DeployTree(ot.Os, revision, origin, nil, kargs, nil)
	if err != nil {
		return err
	}

	deploymentDir := fmt.Sprintf("ostree/deploy/%s/deploy/%s.%d",
		deployment.Osname(), deployment.Csum(), deployment.Deployserial())

	etcDir := path.Join(context.imageMntDir, deploymentDir, "etc")

	err = os.Mkdir(etcDir, 755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	dst, err := os.OpenFile(path.Join(etcDir, "fstab"), os.O_WRONLY|os.O_CREATE, 0755)
	defer dst.Close()
	if err != nil {
		return err
	}

	src, err := os.Open(path.Join(context.imageMntDir, "etc", "fstab"))
	defer src.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	err = sysroot.SimpleWriteDeployment(ot.Os, deployment, nil, 0, nil)
	if err != nil {
		return err
	}

	return nil
}
