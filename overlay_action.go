package main

import (
  "path"
)

type OverlayAction struct {
  source string;
}

func NewOverlayAction(p map[string]interface{}) *OverlayAction {
  overlay := new(OverlayAction)
  overlay.source = p["source"].(string)
  return overlay
}

func (overlay *OverlayAction) Run(context YaibContext) {
  sourcedir := path.Join(context.artifactdir, overlay.source)
  CopyTree(sourcedir, context.rootdir)
}
