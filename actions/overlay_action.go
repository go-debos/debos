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

Optional properties:

- origin -- reference to named file or directory.

- destination -- absolute path in the target rootfs where 'source' will be copied.
Any missing parent directories will be created. All existing files will be overwritten.
If destination isn't set the root of the target rootfs will be used.
*/
package actions

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"

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
		sourceDir := path.Join(context.RecipeDir, overlay.Source)
		if _, err := os.Stat(sourceDir); err != nil {
			return err
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

	source := path.Join(origin, overlay.Source)
	destination, err := debos.RestrictedPath(context.Rootdir, overlay.Destination)
	if err != nil {
		return err
	}

	// Make sure all parts of the destination except the last exists.
	destinationParent := path.Dir(destination)
	err = os.MkdirAll(destinationParent, 0755)
	if err != nil {
		return fmt.Errorf("could not create parent destination path for overlay '%s': %w", destination, err)
	}

	// Copy source into dest
	log.Printf("Overlaying %s on %s", source, destination)
	return debos.CopyTree(source, destination)
}
