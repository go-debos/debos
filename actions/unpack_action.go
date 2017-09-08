/*
Unpack Action

Unpack files from archive to the filesystem.
Useful for creating target rootfs from saved tarball with prepared file structure.

Only (compressed) tar archives are supported currently.

Yaml syntax:
 - action: unpack
   origin: name
   file: file.ext
   compression: gz

Mandatory properties:

- file -- archive's file name. It is possible to skip this property if 'origin'
referenced to downloaded file.

One of the mandatory properties may be omitted with limitations mentioned above.
It is expected to find archive with name pointed in `file` property inside of `origin` in case if both properties are used.

Optional properties:

- origin -- reference to a named file or directory.
The default value is 'artifacts' directory in case if this property is omitted.

- compression -- optional hint for unpack allowing to use proper compression method.

Currently only 'gz', bzip2' and 'xz' compression types are supported.
If not provided an attempt to autodetect the compression type will be done.
*/
package actions

import (
	"fmt"
	"github.com/go-debos/debos"
)

type UnpackAction struct {
	debos.BaseAction `yaml:",inline"`
	Compression      string
	Origin           string
	File             string
}

func (pf *UnpackAction) Verify(context *debos.DebosContext) error {

	if len(pf.Origin) == 0 && len(pf.File) == 0 {
		return fmt.Errorf("Filename can't be empty. Please add 'file' and/or 'origin' property.")
	}

	archive, err := debos.NewArchive(pf.File)
	if err != nil {
		return err
	}
	if len(pf.Compression) > 0 {
		if archive.Type() != debos.Tar {
			return fmt.Errorf("Option 'compression' is supported for Tar archives only.")
		}
		if err := archive.AddOption("tarcompression", pf.Compression); err != nil {
			return fmt.Errorf("'%s': %s", pf.File, err)
		}
	}

	return nil
}

func (pf *UnpackAction) Run(context *debos.DebosContext) error {
	pf.LogStart()
	var origin string

	if len(pf.Origin) > 0 {
		var found bool
		//Trying to get a filename from origins first
		origin, found = context.Origins[pf.Origin]
		if !found {
			return fmt.Errorf("Origin not found '%s'", pf.Origin)
		}
	} else {
		origin = context.Artifactdir
	}

	infile, err := debos.RestrictedPath(origin, pf.File)
	if err != nil {
		return err
	}

	archive, err := debos.NewArchive(infile)
	if err != nil {
		return err
	}
	if len(pf.Compression) > 0 {
		if err := archive.AddOption("tarcompression", pf.Compression); err != nil {
			return err
		}
	}

	return archive.Unpack(context.Rootdir)
}
