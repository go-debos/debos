/*
Overlay Action

Recursive copy of directory or file to target filesystem.

	# Yaml syntax:
	- action: overlay
	  origin: name
	  source: directory
	  destination: directory

Mandatory properties:

- source -- relative path to the directory or file located in path referenced by `origin`.
In case if this property is absent then pure path referenced by 'origin' will be used.
The source property may contain wildcards.

Optional properties:

- origin -- reference to named file or directory.

- destination -- absolute path in the target rootfs where 'source' will be copied.
All existing files will be overwritten.
If destination isn't set '/' of the rootfs will be used.
*/
package actions

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/go-debos/debos"
)

type OverlayAction struct {
	debos.BaseAction `yaml:",inline"`
	Origin           string // origin of overlay, here the export from other action may be used
	Source           string // external path there overlay is
	Destination      string // path inside of rootfs
}

func (overlay *OverlayAction) Verify(context *debos.Context) error {
	if _, err := debos.RestrictedPath(context.Rootdir, overlay.Destination); err != nil {
		return err
	}

	if len(overlay.Source) == 0 && len(overlay.Origin) == 0 {
		return errors.New("'source' and 'origin' properties can't both be empty")
	}

	/* if origin is the recipe, check the path exists on disk */
	if len(overlay.Origin) == 0 || overlay.Origin == "recipe" {
		pattern := debos.CleanPathAt(overlay.Source, context.RecipeDir)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("failed to glob for '%s': %w", pattern, err)
		}

		if len(matches) == 0 {
			return fmt.Errorf("no matches for '%s'", pattern)
		}
	}

	return nil
}

func (overlay *OverlayAction) Run(context *debos.Context) error {
	origin := context.RecipeDir

	//Trying to get a filename from exports first
	if len(overlay.Origin) > 0 {
		var found bool
		if origin, found = context.Origin(overlay.Origin); !found {
			return fmt.Errorf("origin not found '%s'", overlay.Origin)
		}
	}

	destination, err := debos.RestrictedPath(context.Rootdir, overlay.Destination)
	if err != nil {
		return err
	}

	pattern := debos.CleanPathAt(overlay.Source, origin)
	matches, _ := filepath.Glob(pattern)
	for _, source := range matches {
		log.Printf("Overlaying %s on %s", source, destination)
		err := debos.CopyTree(source, destination)
		if err != nil {
			return err
		}
	}

	return nil
}
