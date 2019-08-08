/*
Pkglist action

Export a Debian package list to the artifacts directory

Yaml syntax:
 - action: pkglist
   file: pkglist.txt

Optional properties:

- file -- name of the output file, relative to the artifact directory.
If empty, it defaults to 'pkglist.txt'.
*/
package actions

import (
	"os"
	"path"

	"github.com/go-debos/debos"
)

type PkglistAction struct {
	debos.BaseAction `yaml:",inline"`
	File             string
}

func (pkglist *PkglistAction) Run(context *debos.DebosContext) error {
	pkglist.LogStart()
	var cmdline []string
	var cmd debos.Command
	var err error

	// get the package list inside chroot
	cmd = debos.NewChrootCommandForContext(*context)
	cmdline = []string{"dpkg --list > /output.txt"}
	cmdline = append([]string{"sh", "-c"}, cmdline...)

	err = cmd.Run("pkglist", cmdline...)
	if err != nil {
		return err
	}

	// move it to artifacts folder outside chroot
	src := path.Join(context.Rootdir, "output.txt")
	if pkglist.File == "" {
		pkglist.File = "pkglist.txt"
	}
	dest := path.Join(context.Artifactdir, pkglist.File)

	err = debos.CopyFile(src, dest, 0644)
	if err != nil {
		return err
	}

	err = os.Remove(src)
	if err != nil {
		return err
	}

	return nil
}
