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
		y.Action = &actions.DebootstrapAction{}
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
