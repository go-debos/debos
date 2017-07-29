package main

import (
	"fmt"
	"os"
	"path"
)

type UnpackAction struct {
	*BaseAction
	Compression string
	File        string
}

func (pf *UnpackAction) Run(context *YaibContext) {
	infile := path.Join(context.artifactdir, pf.File)

	os.MkdirAll(context.rootdir, 0755)

	fmt.Printf("Unpacking %s\n", infile)
	err := Command{}.Run("unpack", "tar", "xzf", infile, "-C", context.rootdir)

	if err != nil {
		panic(err)
	}
}
