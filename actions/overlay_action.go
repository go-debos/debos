/*
Overlay Action

Recursive copy of directory or file to target filesystem.

Yaml syntax:
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
All existing files will be overwritten.
If destination isn't set '/' of the rootfs will be used.
*/
package actions

import (
	"fmt"
	"path"

	"github.com/go-debos/debos"
)

type OverlayAction struct {
	debos.BaseAction `yaml:",inline"`
	Origin           string // origin of overlay, here the export from other action may be used
	Source           string // external path there overlay is
	Destination      string // path inside of rootfs
}

func (overlay *OverlayAction) Verify(context *debos.DebosContext) error {
	if _, err := debos.RestrictedPath(context.Rootdir, overlay.Destination); err != nil {
		return err
	}
	return nil
}

func (overlay *OverlayAction) Run(context *debos.DebosContext) error {
	overlay.LogStart()
	origin := context.RecipeDir

	//Trying to get a filename from exports first
	if len(overlay.Origin) > 0 {
		var found bool
		if origin, found = context.Origins[overlay.Origin]; !found {
			return fmt.Errorf("Origin not found '%s'", overlay.Origin)
		}
	}

	sourcedir := path.Join(origin, overlay.Source)
	destination, err := debos.RestrictedPath(context.Rootdir, overlay.Destination)
	if err != nil {
		return err
	}

	return debos.CopyTree(sourcedir, destination)
}
