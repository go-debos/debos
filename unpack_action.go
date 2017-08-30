package main

import (
	"fmt"
	"log"
	"os"
)

type UnpackAction struct {
	BaseAction  `yaml:",inline"`
	Compression string
	Origin      string
	File        string
}

func tarOptions(compression string) string {
	unpackTarOpts := map[string]string{
		"gz":    "-z",
		"bzip2": "-j",
		"xz":    "-J",
	} // Trying to guess all other supported formats

	return unpackTarOpts[compression]
}

func UnpackTarArchive(infile, destination, compression string, options ...string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	log.Printf("Unpacking %s\n", infile)

	command := []string{"tar"}
	command = append(command, options...)
	command = append(command, "-x")
	if unpackTarOpt := tarOptions(compression); len(unpackTarOpt) > 0 {
		command = append(command, unpackTarOpt)
	}
	command = append(command, "-f", infile, "-C", destination)

	return Command{}.Run("unpack", command...)
}

func (pf *UnpackAction) Verify(context *DebosContext) error {

	if len(pf.Origin) == 0 && len(pf.File) == 0 {
		return fmt.Errorf("Filename can't be empty. Please add 'file' and/or 'origin' property.")
	}

	unpackTarOpt := tarOptions(pf.Compression)
	if len(pf.Compression) > 0 && len(unpackTarOpt) == 0 {
		return fmt.Errorf("Compression '%s' is not supported.\n", pf.Compression)
	}

	return nil
}

func (pf *UnpackAction) Run(context *DebosContext) error {
	pf.LogStart()
	var origin string

	if len(pf.Origin) > 0 {
		var found bool
		//Trying to get a filename from origins first
		origin, found = context.origins[pf.Origin]
		if !found {
			return fmt.Errorf("Origin not found '%s'", pf.Origin)
		}
	} else {
		origin = context.artifactdir
	}

	infile, err := RestrictedPath(origin, pf.File)
	if err != nil {
		return err
	}

	return UnpackTarArchive(infile, context.rootdir, pf.Compression)
}
