package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

type RawAction struct {
	BaseAction `yaml:",inline"`
	Offset     string
	Source     string
	Path       string
}

func (raw *RawAction) Verify(context *YaibContext) error {
	if raw.Source != "rootdir" {
		return errors.New("Only suppport sourcing from filesystem")
	}

	return nil
}

func (raw *RawAction) Run(context *YaibContext) error {
	s := path.Join(context.rootdir, raw.Path)
	content, err := ioutil.ReadFile(s)

	if err != nil {
		return fmt.Errorf("Failed to read %s", s)
	}

	target, err := os.OpenFile(context.image, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Failed to open image file %v", err)
	}

	offset, err := strconv.ParseInt(raw.Offset, 0, 64)
	if err != nil {
		return fmt.Errorf("Couldn't parse offset %v", err)
	}
	bytes, err := target.WriteAt(content, offset)
	if bytes != len(content) {
		return errors.New("Couldn't write complete data")
	}

	return nil
}
