package main

import (
	"path"
)

type OverlayAction struct {
	BaseAction `yaml:",inline"`
	Source     string
}

func (overlay *OverlayAction) Run(context *DebosContext) error {
	overlay.LogStart()
	sourcedir := path.Join(context.recipeDir, overlay.Source)
	return CopyTree(sourcedir, context.rootdir)
}
