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
