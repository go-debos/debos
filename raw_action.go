package main

import (
	"os"
	"log"
	"strconv"
	"path"
	"io/ioutil"
)

type RawAction struct {
	*BaseAction
	Offset string
	Source string
	Path string
}

func (raw *RawAction) Verify(context YaibContext) {
	if raw.Source != "rootdir" {
		log.Fatal("Only suppport sourcing from filesystem")
	}
}

func (raw *RawAction) Run(context *YaibContext) {
	s := path.Join(context.rootdir, raw.Path)
	content, err := ioutil.ReadFile(s)

	if err != nil {
		log.Fatalf("Failed to read %s\n", s)
	}

	target, err := os.OpenFile(context.image, os.O_WRONLY, 0)
	if err != nil {
		log.Fatalf("Failed to open image file %v\n", err)
	}

  offset, err := strconv.ParseInt(raw.Offset, 0, 64)
	if err != nil {
		log.Fatalf("Couldn't parse offset %v\n", err)
	}
	bytes, err := target.WriteAt(content, offset)
	if bytes != len(content) {
		log.Fatal("Couldn't write complte data")
	}
}
