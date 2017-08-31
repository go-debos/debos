package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
)

type RawAction struct {
	BaseAction `yaml:",inline"`
	Origin     string // there the source comes from
	Offset     string
	Source     string // relative path inside of origin
	Path       string // deprecated option (for backward compatibility)
}

func (raw *RawAction) checkDeprecatedSyntax() error {

	// New syntax is based on 'origin' and 'source'
	// Check if we do not mix new and old syntax
	// TODO: remove deprecated syntax verification
	if len(raw.Path) > 0 {
		// Deprecated syntax based on 'source' and 'path'
		log.Printf("Usage of 'source' and 'path' properties is deprecated.")
		log.Printf("Please use 'origin' and 'source' properties.")
		if len(raw.Origin) > 0 {
			return errors.New("Can't mix 'origin' and 'path'(deprecated option) properties")
		}
		if len(raw.Source) == 0 {
			return errors.New("'source' and 'path' properties can't be empty")
		}
		// Switch to new syntax
		raw.Origin = raw.Source
		raw.Source = raw.Path
		raw.Path = ""
	}
	return nil
}

func (raw *RawAction) Verify(context *DebosContext) error {
	if err := raw.checkDeprecatedSyntax(); err != nil {
		return err
	}

	if len(raw.Origin) == 0 || len(raw.Source) == 0 {
		return errors.New("'origin' and 'source' properties can't be empty")
	}

	return nil
}

func (raw *RawAction) Run(context *DebosContext) error {
	raw.LogStart()
	origin, found := context.origins[raw.Origin]
	if !found {
		return fmt.Errorf("Origin `%s` doesn't exist\n", raw.Origin)
	}
	s := path.Join(origin, raw.Source)
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
