/*
Pack Action

Create tarball with filesystem.

Yaml syntax:
 - action: pack
   file: filename.ext
   compression: gz

Mandatory properties:

- file -- name of the output tarball, relative to the artifact directory.

- compression -- compression type to use. Only 'gz' is supported at the moment.

*/
package actions

import (
	"log"
	"path"

	"github.com/go-debos/debos"
)

type PackAction struct {
	debos.BaseAction `yaml:",inline"`
	Compression      string
	File             string
}

func (pf *PackAction) Run(context *debos.DebosContext) error {
	pf.LogStart()
	outfile := path.Join(context.Artifactdir, pf.File)

	log.Printf("Compressing to %s\n", outfile)
	return debos.Command{}.Run("Packing", "tar", "czf", outfile,
		"--xattrs", "--xattrs-include=*.*",
		"-C", context.Rootdir, ".")
}
