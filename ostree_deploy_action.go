package main

import (
	"log"
	"path"

	ostree "github.com/sjoerdsimons/ostree-go/pkg/otbuiltin"
)

type OstreeDeployAction struct {
	*BaseAction
	Repository       string
	RemoteRepository string
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

	origin := sysroot.OriginNewFromRefspec(ot.Branch)
	deployment, err := sysroot.DeployTree(ot.Os, revision, origin, nil, nil, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}

	err = sysroot.SimpleWriteDeployment(ot.Os, deployment, nil, 0, nil)
	if err != nil {
		log.Fatalf("%s", err)
	}
}
