package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/sjoerdsimons/ostree-go/pkg/otbuiltin"
)

type OstreeCommitAction struct {
	*BaseAction
	Repository string
	Branch     string
	Subject    string
	Command    string
}

func emptyDir(dir string) {
	d, _ := os.Open(repoDev)
	defer d.Close()
	files, err := d.Readdirnames(-1)
	for _, f := range files {
		os.RemoveAll(f)
	}
}

func (ot *OstreeCommitAction) Run(context *YaibContext) {
	repoPath := path.Join(context.artifactdir, ot.Repository)

	emptyDir(path.Join(context.rootdir, "dev"))

	repo, err := otbuiltin.OpenRepo(repoPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = repo.PrepareTransaction()
	if err != nil {
		log.Fatal(err)
	}

	opts := otbuiltin.NewCommitOptions()
	opts.Subject = ot.Subject
	ret, err := repo.Commit(context.rootdir, ot.Branch, opts)
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("Commit: %s\n", ret)
	}
	_, err = repo.CommitTransaction()
	if err != nil {
		log.Fatal(err)
	}
}
