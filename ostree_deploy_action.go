package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
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

func (ot *OstreeDeployAction) Run(context *YaibContext) {
	repoPath := "file://" + path.Join(context.artifactdir, ot.Repository)

	sysroot := ostree.NewSysroot(context.imageMntDir)
	err := sysroot.InitializeFS()
	if err != nil {
		log.Fatal(err)
	}

	err = sysroot.InitOsname(ot.Os, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	dstRepo, err := sysroot.Repo(nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	err = dstRepo.RemoteAdd("origin", ot.RemoteRepository, nil, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	var options ostree.PullOptions
	options.OverrideRemoteName = "origin"
	options.Refs = []string{ot.Branch}

	err = dstRepo.PullWithOptions(repoPath, options, nil, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	/* Required by ostree to make sure a bunch of information was pulled in  */
	sysroot.Load(nil)

	revision, err := dstRepo.ResolveRev(ot.Branch, false)
	if err != nil {
		log.Fatalf("%s", err)
	}

	cmdline, _ := ioutil.ReadFile(path.Join(context.imageMntDir, "etc/kernel/cmdline"))
	kargs := strings.Split(strings.TrimSpace(string(cmdline)), " ")

	origin := sysroot.OriginNewFromRefspec("origin:" + ot.Branch)
	deployment, err := sysroot.DeployTree(ot.Os, revision, origin, nil, kargs, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	deploymentDir := fmt.Sprintf("ostree/deploy/%s/deploy/%s.%d",
		deployment.Osname(), deployment.Csum(), deployment.Deployserial())

	etcDir := path.Join(context.imageMntDir, deploymentDir, "etc")

	err = os.Mkdir(etcDir, 755)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("%s", err)
	}

	dst, err := os.OpenFile(path.Join(etcDir, "fstab"), os.O_WRONLY|os.O_CREATE, 0755)
	defer dst.Close()
	if err != nil {
		log.Fatalf("%s", err)
	}

	src, err := os.Open(path.Join(context.imageMntDir, "etc", "fstab"))
	defer src.Close()
	if err != nil {
		log.Fatalf("%s", err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		log.Fatalf("%s", err)
	}

	err = sysroot.SimpleWriteDeployment(ot.Os, deployment, nil, 0, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}
}
