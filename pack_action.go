package main

import (
	"log"
	"path"
)

type PackAction struct {
	*BaseAction
	Compression string
	File        string
}

func (pf *PackAction) Run(context *YaibContext) error {
	outfile := path.Join(context.artifactdir, pf.File)

	log.Printf("Compression to %s\n", outfile)
	return Command{}.Run("Packing", "tar", "czf", outfile, "-C", context.rootdir, ".")
}
