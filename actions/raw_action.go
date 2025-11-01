/*
Raw Action

Directly write a file to the output image at a given offset.
This is typically useful for bootloaders.

	# Yaml syntax:
	- action: raw
	  origin: name
	  source: filename
	  offset: bytes

Mandatory properties:

- source -- the name of file to be written into the output image.

Optional properties:

- origin -- reference to named file or directory.
If not provided, defaults to the recipe directory.

- offset -- offset in bytes or in sector number e.g 256s.
The sector size is either the recipe header 'sectorsize' or the default 512 sector
size.
Internal templating mechanism will append the 's' suffix, for instance: '{{ sector 256 }}' will be converted to '256s'.
Deprecated, use '256s' instead of '{{ sector 256 }}'.
The default value is zero.

- partition -- named partition to write to
*/
package actions

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

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
			return errors.New("can't mix 'origin' and 'path'(deprecated option) properties")
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

func (raw *RawAction) Verify(_ *debos.Context) error {
	if err := raw.checkDeprecatedSyntax(); err != nil {
		return err
	}

	if len(raw.Source) == 0 {
		return errors.New("'source' property can't be empty")
	}

	return nil
}

func (raw *RawAction) Run(context *debos.Context) error {
	origin := context.RecipeDir

	if len(raw.Origin) > 0 {
		var found bool
		if origin, found = context.Origin(raw.Origin); !found {
			return fmt.Errorf("origin `%s` doesn't exist", raw.Origin)
		}
	}

	s := path.Join(origin, raw.Source)
	content, err := os.ReadFile(s)

	if err != nil {
		return fmt.Errorf("failed to read %s", s)
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
			return fmt.Errorf("failed to find partition named %s", raw.Partition)
		}
	} else {
		devicePath = context.Image
	}

	target, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", devicePath, err)
	}
	defer target.Close()

	var offset int64
	if len(raw.Offset) > 0 {
		sector := false
		offs := raw.Offset
		if strings.HasSuffix(offs, "s") {
			sector = true
			offs = strings.TrimSuffix(offs, "s")
		}
		offset, err = strconv.ParseInt(offs, 0, 64)
		if err != nil {
			return fmt.Errorf("couldn't parse offset %w", err)
		}

		if sector {
			offset = offset * int64(context.SectorSize)
		}
	}

	bytes, err := target.WriteAt(content, offset)
	if bytes != len(content) {
		return fmt.Errorf("couldn't write complete data %w", err)
	}

	err = target.Sync()
	if err != nil {
		return fmt.Errorf("couldn't sync content %w", err)
	}

	return nil
}
