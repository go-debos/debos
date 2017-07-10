package main

import (
	"fmt"
	"log"
	"path"
)

type PackAction struct {
	Compression string
	Target      string
}

func (pf *PackAction) Run(context YaibContext) {
	outfile := path.Join(context.artifactdir, pf.Target)

	fmt.Printf("Compression to %s\n", outfile)
	err := RunCommand("Packing", "tar", "czf", outfile, "-C", context.rootdir, ".")

	if err != nil {
		log.Panic(err)
	}
}
