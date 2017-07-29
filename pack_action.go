package main

import (
	"fmt"
	"log"
	"path"
)

type PackAction struct {
	*BaseAction
	Compression string
	File        string
}

func (pf *PackAction) Run(context *YaibContext) {
	outfile := path.Join(context.artifactdir, pf.File)

	fmt.Printf("Compression to %s\n", outfile)
	err := Command{}.Run("Packing", "tar", "czf", outfile, "-C", context.rootdir, ".")

	if err != nil {
		log.Panic(err)
	}
}
