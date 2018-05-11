/*
OstreeCommit Action

Create OSTree commit from rootfs.

Yaml syntax:
 - action: ostree-commit
   repository: repository name
   branch: branch name
   subject: commit message

Mandatory properties:

- repository -- path to repository with OSTree structure; the same path is
used by 'ostree' tool with '--repo' argument.
This path is relative to 'artifact' directory.
Please keep in mind -- you will need a root privileges for 'bare' repository
type (https://ostree.readthedocs.io/en/latest/manual/repo/#repository-types-and-locations).

- branch -- OSTree branch name that should be used for the commit.

Optional properties:

- subject -- one line message with commit description.
*/
package actions

import (
	"log"
	"os"
	"path"

	"github.com/go-debos/debos"
	"github.com/sjoerdsimons/ostree-go/pkg/otbuiltin"
)

type OstreeCommitAction struct {
	debos.BaseAction `yaml:",inline"`
	Repository       string
	Branch           string
	Subject          string
	Command          string
}

func emptyDir(dir string) {
	d, _ := os.Open(dir)
	defer d.Close()

	files, err := d.Readdirnames(-1)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		err := os.RemoveAll(path.Join(dir, f))
		if err != nil {
	                log.Fatalf("Failed to remove file: %v", err)
		}
	}
}

func (ot *OstreeCommitAction) Run(context *debos.DebosContext) error {
	ot.LogStart()
	repoPath := path.Join(context.Artifactdir, ot.Repository)

	emptyDir(path.Join(context.Rootdir, "dev"))

	repo, err := otbuiltin.OpenRepo(repoPath)
	if err != nil {
		return err
	}

	_, err = repo.PrepareTransaction()
	if err != nil {
		return err
	}

	opts := otbuiltin.NewCommitOptions()
	opts.Subject = ot.Subject
	ret, err := repo.Commit(context.Rootdir, ot.Branch, opts)
	if err != nil {
		return err
	} else {
		log.Printf("Commit: %s\n", ret)
	}
	_, err = repo.CommitTransaction()
	if err != nil {
		return err
	}

	return nil
}
