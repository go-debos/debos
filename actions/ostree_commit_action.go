/*
OstreeCommit Action

Create OSTree commit from rootfs.

	# Yaml syntax:
	- action: ostree-commit
	  repository: repository name
	  branch: branch name
	  subject: commit message
	  collection-id: org.apertis.example
	  ref-binding:
	    - branch1
	    - branch2
	  metadata:
	    key: value
	    vendor.key: somevalue

Mandatory properties:

- repository -- path to repository with OSTree structure; the same path is
used by 'ostree' tool with '--repo' argument.
This path is relative to 'artifact' directory.
Please keep in mind -- you will need a root privileges for 'bare' repository
type (https://ostree.readthedocs.io/en/latest/manual/repo/#repository-types-and-locations).

- branch -- OSTree branch name that should be used for the commit.

Optional properties:

- subject -- one line message with commit description.

- collection-id -- Collection ID ref binding (requires libostree 2018.6).

  - ref-binding -- enforce that the commit was retrieved from one of the branch names in this array.
    If 'collection-id' is set and 'ref-binding' is empty, will default to the branch name.

- metadata -- key-value pairs of meta information to be added into commit.
*/
package actions

import (
	"fmt"
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
	CollectionID     string   `yaml:"collection-id"`
	RefBinding       []string `yaml:"ref-binding"`
	Metadata         map[string]string
}

func emptyDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()

	files, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", dir, err)
	}

	for _, f := range files {
		if err := os.RemoveAll(path.Join(dir, f)); err != nil {
			return fmt.Errorf("remove %s: %w", f, err)
		}
	}
	return nil
}

func (ot *OstreeCommitAction) Run(context *debos.Context) error {
	repoPath := path.Join(context.Artifactdir, ot.Repository)

	if err := emptyDir(path.Join(context.Rootdir, "dev")); err != nil {
		return fmt.Errorf("empty dev dir: %w", err)
	}

	repo, err := otbuiltin.OpenRepo(repoPath)
	if err != nil {
		return fmt.Errorf("open ostree repo %s: %w", repoPath, err)
	}

	_, err = repo.PrepareTransaction()
	if err != nil {
		return fmt.Errorf("prepare ostree transaction: %w", err)
	}

	opts := otbuiltin.NewCommitOptions()
	opts.Subject = ot.Subject
	for k, v := range ot.Metadata {
		str := fmt.Sprintf("%s=%s", k, v)
		opts.AddMetadataString = append(opts.AddMetadataString, str)
	}

	if ot.CollectionID != "" {
		opts.CollectionID = ot.CollectionID
		if len(ot.RefBinding) == 0 {
			// Add current branch if not explitely set via 'ref-binding'
			opts.RefBinding = append(opts.RefBinding, ot.Branch)
		}
	}

	// Add values from 'ref-binding' if any
	opts.RefBinding = append(opts.RefBinding, ot.RefBinding...)

	ret, err := repo.Commit(context.Rootdir, ot.Branch, opts)
	if err != nil {
		return fmt.Errorf("commit ostree repo: %w", err)
	}
	log.Printf("Commit: %s\n", ret)
	_, err = repo.CommitTransaction()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
