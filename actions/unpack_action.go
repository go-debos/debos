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

	unpackTarOpt := debos.TarOptions(pf.Compression)
	if len(pf.Compression) > 0 && len(unpackTarOpt) == 0 {
		return fmt.Errorf("Compression '%s' is not supported.\n", pf.Compression)
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

	return debos.UnpackTarArchive(infile, context.Rootdir, pf.Compression)
}
