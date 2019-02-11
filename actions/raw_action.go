/*
Raw Action

Directly write a file to the output image at a given offset.
This is typically useful for bootloaders.

Yaml syntax:
 - action: raw
   origin: name
   source: filename
   offset: bytes

Mandatory properties:

- origin -- reference to named file or directory.

- source -- the name of file located in 'origin' to be written into the output image.

Optional properties:

- offset -- offset in bytes for output image file.
It is possible to use internal templating mechanism of debos to calculate offset
with sectors (512 bytes) instead of bytes, for instance: '{{ sector 256 }}'.
The default value is zero.

- partition -- named partition to write to
*/
package actions

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"

	"github.com/go-debos/debos"
)

type RawAction struct {
	debos.BaseAction `yaml:",inline"`
	Origin           string // there the source comes from
	Offset           string
	Source           string // relative path inside of origin
	Path             string // deprecated option (for backward compatibility)
	Partition        string // Partition to write otherwise full image
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

func (raw *RawAction) Verify(context *debos.DebosContext) error {
	if err := raw.checkDeprecatedSyntax(); err != nil {
		return err
	}

	if len(raw.Origin) == 0 || len(raw.Source) == 0 {
		return errors.New("'origin' and 'source' properties can't be empty")
	}

	return nil
}

func (raw *RawAction) Run(context *debos.DebosContext) error {
	raw.LogStart()
	origin, found := context.Origins[raw.Origin]
	if !found {
		return fmt.Errorf("Origin `%s` doesn't exist\n", raw.Origin)
	}
	s := path.Join(origin, raw.Source)
	content, err := ioutil.ReadFile(s)

	if err != nil {
		return fmt.Errorf("Failed to read %s", s)
	}

	var devicePath string
	if raw.Partition != "" {
		for _, p := range context.ImagePartitions {
			if p.Name == raw.Partition {
				devicePath = p.DevicePath
				break
			}
		}

		if devicePath == "" {
			return fmt.Errorf("Failed to find partition named %s", raw.Partition)
		}
	} else {
		devicePath = context.Image
	}

	target, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Failed to open %s: %v", devicePath, err)
	}
	defer target.Close()

	offset, err := strconv.ParseInt(raw.Offset, 0, 64)
	if err != nil {
		return fmt.Errorf("Couldn't parse offset %v", err)
	}
	bytes, err := target.WriteAt(content, offset)
	if bytes != len(content) {
		return fmt.Errorf("Couldn't write complete data %v", err)
	}

	err = target.Sync()
	if err != nil {
		return fmt.Errorf("Couldn't sync content %v", err)
	}

	return nil
}
