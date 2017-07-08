package main

import (
	"fmt"
	"os"
	"path"
)

type UnpackAction struct {
	compression string
	source      string
}

func NewUnpackAction(p map[string]interface{}) *UnpackAction {
	pf := new(UnpackAction)
	pf.source = p["source"].(string)
	pf.compression = p["compression"].(string)
	return pf
}

func (pf *UnpackAction) Run(context YaibContext) {
	infile := path.Join(context.artifactdir, pf.source)

	os.MkdirAll(context.rootdir, 0755)

	fmt.Printf("Unpacking %s\n", infile)
	err := RunCommand("unpack", "tar", "xzf", infile, "-C", context.rootdir)

	if err != nil {
		panic(err)
	}
}
