package main

import (
	"log"
	"path"
)

type PackAction struct {
	BaseAction  `yaml:",inline"`
	Compression string
	File        string
}

func (pf *PackAction) Run(context *DebosContext) error {
	pf.LogStart()
	outfile := path.Join(context.artifactdir, pf.File)

	log.Printf("Compression to %s\n", outfile)
	return Command{}.Run("Packing", "tar", "czf", outfile, "-C", context.rootdir, ".")
}
