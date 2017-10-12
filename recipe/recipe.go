/*
Package 'recipe' implements actions mapping to YAML recipe.

Recipe syntax

Recipe is a YAML file which is pre-processed though Golang
text templating engine (https://golang.org/pkg/text/template)

Recipe is composed of 2 parts:

- header

- actions

Comments are allowed and should be prefixed with '#' symbol.

 # Declare variable 'Var'
 {{- $Var := "Value" -}}

 # Header
 architecture: arm64

 # Actions are executed in listed order
 actions:
   - action: ActionName1
     property1: true

   - action: ActionName2
     # Use value of variable 'Var' defined above
     property2: {{$Var}}

Mandatory properties for receipt:

- architecture -- target architecture

- actions -- at least one action should be listed

Supported actions

- apt -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Apt_Action

- debootstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Debootstrap_Action

- download -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Download_Action

- filesystem-deploy -- https://godoc.org/github.com/go-debos/debos/actions#hdr-FilesystemDeploy_Action

- image-partition -- https://godoc.org/github.com/go-debos/debos/actions#hdr-ImagePartition_Action

- ostree-commit -- https://godoc.org/github.com/go-debos/debos/actions#hdr-OstreeCommit_Action

- ostree-deploy -- https://godoc.org/github.com/go-debos/debos/actions#hdr-OstreeDeploy_Action

- overlay -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Overlay_Action

- pack -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Pack_Action

- raw -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Raw_Action

- run -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Run_Action

- unpack -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Unpack_Action
*/
package recipe

import (
	"bytes"
	"fmt"
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"gopkg.in/yaml.v2"
	"path"
	"text/template"
)

/* the YamlAction just embed the Action interface and implements the
 * UnmarshalYAML function so it can select the concrete implementer of a
 * specific action at unmarshaling time */
type YamlAction struct {
	debos.Action
}

type Recipe struct {
	Architecture string
	Actions      []YamlAction
}

func (y *YamlAction) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var aux debos.BaseAction

	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	switch aux.Action {
	case "debootstrap":
		y.Action = actions.NewDebootstrapAction()
	case "pack":
		y.Action = &actions.PackAction{}
	case "unpack":
		y.Action = &actions.UnpackAction{}
	case "run":
		y.Action = &actions.RunAction{}
	case "apt":
		y.Action = &actions.AptAction{}
	case "ostree-commit":
		y.Action = &actions.OstreeCommitAction{}
	case "ostree-deploy":
		y.Action = actions.NewOstreeDeployAction()
	case "overlay":
		y.Action = &actions.OverlayAction{}
	case "image-partition":
		y.Action = &actions.ImagePartitionAction{}
	case "filesystem-deploy":
		y.Action = actions.NewFilesystemDeployAction()
	case "raw":
		y.Action = &actions.RawAction{}
	case "download":
		y.Action = &actions.DownloadAction{}
	default:
		return fmt.Errorf("Unknown action: %v", aux.Action)
	}

	unmarshal(y.Action)

	return nil
}

func sector(s int) int {
	return s * 512
}

/*
Parse method reads YAML recipe file and map all steps to appropriate actions.

- file -- is the path to configuration file

- templateVars -- optional argument allowing to use custom map for templating
engine. Multiple template maps have no effect; only first map will be used.
*/
func (r *Recipe) Parse(file string, templateVars ...map[string]string) error {
	t := template.New(path.Base(file))
	funcs := template.FuncMap{
		"sector": sector,
	}
	t.Funcs(funcs)

	if _, err := t.ParseFiles(file); err != nil {
		return err
	}

	if len(templateVars) == 0 {
		templateVars = append(templateVars, make(map[string]string))
	}

	data := new(bytes.Buffer)
	if err := t.Execute(data, templateVars[0]); err != nil {
		return err
	}

	if err := yaml.Unmarshal(data.Bytes(), &r); err != nil {
		return err
	}

	if len(r.Architecture) == 0 {
		return fmt.Errorf("Recipe file must have 'architecture' property")
	}

	if len(r.Actions) == 0 {
		return fmt.Errorf("Recipe file must have at least one action")
	}

	return nil
}
