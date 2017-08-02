package main

import (
	"log"
	"os"
	"path"
)

type UnpackAction struct {
	BaseAction  `yaml:",inline"`
	Compression string
	File        string
}

func (pf *UnpackAction) Run(context *DebosContext) error {
	pf.LogStart()
	infile := path.Join(context.artifactdir, pf.File)

	os.MkdirAll(context.rootdir, 0755)

	log.Printf("Unpacking %s\n", infile)
	return Command{}.Run("unpack", "tar", "xzf", infile, "-C", context.rootdir)
}
