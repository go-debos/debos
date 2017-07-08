package main

import (
  "fmt"
  "path"
  "log"
)

type PackAction struct {
  compression string;
  target string;
}

func NewPackAction(p map[string]interface{}) *PackAction {
  pf := new(PackAction)
  pf.target = p["target"].(string)
  pf.compression = p["compression"].(string)
  return pf
}

func (pf *PackAction) Run(context YaibContext) {
  outfile := path.Join(context.artifactdir, pf.target)

  fmt.Printf("Compression to %s\n", outfile)
  err := RunCommand("Packing", "tar", "czf", outfile, "-C", context.rootdir, ".")

  if err != nil {
    log.Panic(err)
  }
}
