/*
Pack Action

Create tarball with filesystem.

Yaml syntax:
 - action: pack
   file: filename.ext
   compression: gz

Mandatory properties:

- file -- name of the output tarball, relative to the artifact directory.

Optional properties:

- compression -- compression type to use. Currently 'gz', 'bzip2', 'lzip', lzma', 'lzop', 'xz', 'zstd' compression types
are supported. Use 'none' for uncompressed tarball. Use 'auto' to pick via file extension. The 'gz' compression type will
be used by default.

*/
package actions

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

var tarOpts = map[string]string{
	"bzip2": "--bzip2",
	"gz":    "--gzip",
	"lzop": "--lzop",
	"lzma": "--lzma",
	"lzip", "--lzip",
	"xz":    "--xz",
	"zstd":  "--zstd",
	"auto", "--auto-compress",
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
	outfile := path.Join(context.Artifactdir, pf.File)

	var compressOpt = tarOpts[pf.Compression]
	log.Printf("Compressing to %s\n", outfile)
	return debos.Command{}.Run("Packing", "tar", "cf", compressOpt outfile,
		"--xattrs", "--xattrs-include=*.*",
		"-C", context.Rootdir, ".")
}
