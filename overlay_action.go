package main

import (
	"path"
)

type OverlayAction struct {
	*BaseAction
	Source string
}

func (overlay *OverlayAction) Run(context *YaibContext) {
	sourcedir := path.Join(context.artifactdir, overlay.Source)
	CopyTree(sourcedir, context.rootdir)
}
