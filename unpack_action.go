package main

import (
	"fmt"
	"os"
	"path"
)

type UnpackAction struct {
	*BaseAction
	Compression string
	Source      string
}

func (pf *UnpackAction) Run(context *YaibContext) {
	infile := path.Join(context.artifactdir, pf.Source)

	os.MkdirAll(context.rootdir, 0755)

	fmt.Printf("Unpacking %s\n", infile)
	err := RunCommand("unpack", "tar", "xzf", infile, "-C", context.rootdir)

	if err != nil {
		panic(err)
	}
}
