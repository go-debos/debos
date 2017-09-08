package recipe

import (
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"log"
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
		log.Fatalf("Unknown action: %v", aux.Action)
	}

	unmarshal(y.Action)

	return nil
}
