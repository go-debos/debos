package main

import (
	"fmt"
	"log"
	"os"
	"path"
)

type UnpackAction struct {
	BaseAction  `yaml:",inline"`
	Compression string
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
	unpackTarOpt := tarOptions(pf.Compression)
	if len(pf.Compression) > 0 && len(unpackTarOpt) == 0 {
		return fmt.Errorf("Compression '%s' is not supported.\n", pf.Compression)
	}

	return nil
}

func (pf *UnpackAction) Run(context *DebosContext) error {
	pf.LogStart()
	infile := path.Join(context.artifactdir, pf.File)

	return UnpackTarArchive(infile, context.rootdir, pf.Compression)
}
