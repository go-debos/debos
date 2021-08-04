/*
Pack Action

Create tarball with filesystem.

 # Yaml syntax:
 - action: pack
   file: filename.ext
   compression: gz

Mandatory properties:

- file -- name of the output tarball, relative to the artifact directory.

Optional properties:

- compression -- compression type to use. Currently only 'gz', 'bzip2' and 'xz'
compression types are supported. Use 'none' for uncompressed tarball. The 'gz'
compression type will be used by default.

*/
package actions

import (
	"fmt"
	"log"
	"path"
	"strings"
	"os/exec"

	"github.com/go-debos/debos"
)

var tarOpts = map[string]string{
	"gz":    "z",
	"bzip2": "j",
	"xz":    "J",
	"none":  "",
}

type PackAction struct {
	debos.BaseAction `yaml:",inline"`
	Compression      string
	File             string
}

func NewPackAction() *PackAction {
	d := PackAction{}
	// Use gz by default
	d.Compression = "gz"

	return &d
}

func (pf *PackAction) Verify(context *debos.DebosContext) error {
	_, compressionAvailable := tarOpts[pf.Compression]
	if compressionAvailable {
		return nil
	}

	possibleTypes := make([]string, 0, len(tarOpts))
	for key := range tarOpts {
		possibleTypes = append(possibleTypes, key)
	}

	return fmt.Errorf("Option 'compression' has an unsupported type: `%s`. Possible types are %s.",
		pf.Compression, strings.Join(possibleTypes, ", "))
}

func (pf *PackAction) Run(context *debos.DebosContext) error {
	pf.LogStart()
	usePigz := false
	if pf.Compression == "gz" {
		if _,err := exec.LookPath("pigz"); err == nil {
			usePigz = true
		}
	}
	outfile := path.Join(context.Artifactdir, pf.File)

	command := []string{"tar"}
	if usePigz == true {
		command = append(command, "cf")
	} else {
		command = append(command, "cf" + tarOpts[pf.Compression])
	}
	command = append(command, outfile)
	command = append(command, "--xattrs")
	command = append(command, "--xattrs-include=*.*")
	if usePigz == true {
		command = append(command, "--use-compress-program=pigz")
	}
	command = append(command, "-C", context.Rootdir)
	command = append(command, ".")

	log.Printf("Compressing to %s\n", outfile)
	return debos.Command{}.Run("Packing", command...)
}
