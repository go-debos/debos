package main

import (
	"fmt"
	"path"
)

type OverlayAction struct {
	BaseAction  `yaml:",inline"`
	Origin      string // origin of overlay, here the export from other action may be used
	Source      string // external path there overlay is
	Destination string // path inside of rootfs
}

func (overlay *OverlayAction) Verify(context *DebosContext) error {
	if _, err := RestrictedPath(context.rootdir, overlay.Destination); err != nil {
		return err
	}
	return nil
}

func (overlay *OverlayAction) Run(context *DebosContext) error {
	overlay.LogStart()
	origin := context.recipeDir

	//Trying to get a filename from exports first
	if len(overlay.Origin) > 0 {
		var found bool
		if origin, found = context.origins[overlay.Origin]; !found {
			return fmt.Errorf("Origin not found '%s'", overlay.Origin)
		}
	}

	sourcedir := path.Join(origin, overlay.Source)
	destination, err := RestrictedPath(context.rootdir, overlay.Destination)
	if err != nil {
		return err
	}

	return CopyTree(sourcedir, destination)
}
