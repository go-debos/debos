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

- compression -- compression type to use. Currently 'bzip2', 'gz', 'lzip', lzma', 'lzop',
'xz' and 'zstd' compression types are supported. Use 'none' for uncompressed tarball.
Use 'auto' to pick via file extension. The 'gz' compression type will be used by default.
*/
package actions

import (
	"fmt"
	"log"
	"os/exec"
	"path"
	"strings"

	"github.com/go-debos/debos"
)

var tarOpts = map[string]string{
	"bzip2": "--bzip2",
	"gz":    "--gzip",
	"lzip":  "--lzip",
	"lzma":  "--lzma",
	"lzop":  "--lzop",
	"xz":    "--xz",
	"zstd":  "--zstd",
	"auto":  "--auto-compress",
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

func (pf *PackAction) Verify(_ *debos.Context) error {
	_, compressionAvailable := tarOpts[pf.Compression]
	if compressionAvailable {
		return nil
	}

	possibleTypes := make([]string, 0, len(tarOpts))
	for key := range tarOpts {
		possibleTypes = append(possibleTypes, key)
	}

	return fmt.Errorf("option 'compression' has an unsupported type: `%s`; possible types are %s",
		pf.Compression, strings.Join(possibleTypes, ", "))
}

func (pf *PackAction) CheckEnvironment(_ *debos.Context) error {
	cmd := debos.Command{}
	if err := cmd.CheckExecutableExists("tar"); err != nil {
		return err
	}
	return nil
}

func (pf *PackAction) Run(context *debos.Context) error {
	usePigz := false
	if pf.Compression == "gz" {
		if _, err := exec.LookPath("pigz"); err == nil {
			usePigz = true
		}
	}
	outfile := path.Join(context.Artifactdir, pf.File)

	command := []string{"tar"}
	command = append(command, "cf")
	command = append(command, outfile)
	command = append(command, "--xattrs")
	command = append(command, "--xattrs-include=*.*")
	if usePigz {
		command = append(command, "--use-compress-program=pigz")
	} else if tarOpts[pf.Compression] != "" {
		command = append(command, tarOpts[pf.Compression])
	}
	command = append(command, "-C", context.Rootdir)
	command = append(command, ".")

	log.Printf("Compressing to %s\n", outfile)
	return debos.Command{}.Run("Packing", command...)
}
