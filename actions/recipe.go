/*
Package 'recipe' implements actions mapping to YAML recipe.

# Recipe syntax

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
	sectorsize: 512

	# Actions are executed in listed order
	actions:
	  - action: ActionName1
	    property1: true

	  - action: ActionName2
	    # Use value of variable 'Var' defined above
	    property2: {{$Var}}

The following custom template functions are available:

- sector: Returns the argument with 's' suffix for raw action` (Deprecated)
- escape: Shell escape the  argument `{{ escape $var }}`
- uuid5: Generates fixed UUID value `{{ uuid5 $random-uuid $text }}`
- functions from [slim-sprig](https://go-task.github.io/slim-sprig/)

Mandatory properties for recipe:

- architecture -- target architecture

- actions -- at least one action should be listed

Optional properties for recipe:

- sectorsize: Overrides the default 512 bytes sectorsize, mandatory for device using 4k block size such as UFS or NVMe storage. Setting the sectorsize to an
other value than '512' is not supported by the 'uml' fakemachine backend.

# Supported actions

- apt -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Apt_Action

- debootstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Debootstrap_Action

- mmdebstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Mmdebstrap_Action

- download -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Download_Action

- filesystem-deploy -- https://godoc.org/github.com/go-debos/debos/actions#hdr-FilesystemDeploy_Action

- image-partition -- https://godoc.org/github.com/go-debos/debos/actions#hdr-ImagePartition_Action

- ostree-commit -- https://godoc.org/github.com/go-debos/debos/actions#hdr-OstreeCommit_Action

- ostree-deploy -- https://godoc.org/github.com/go-debos/debos/actions#hdr-OstreeDeploy_Action

- overlay -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Overlay_Action

- pack -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Pack_Action

- pacman -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Pacman_Action

- pacstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Pacstrap_Action

- raw -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Raw_Action

- recipe -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Recipe_Action

- run -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Run_Action

- unpack -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Unpack_Action
*/
package actions

import (
	"al.essio.dev/pkg/shellescape"
	"bytes"
	"fmt"
	"github.com/go-debos/debos"
	"github.com/go-task/slim-sprig/v3"
	"github.com/goccy/go-yaml"
	"log"
	"path"
	"reflect"
	"strconv"
	"strings"
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
	SectorSize   int
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
		y.Action = NewDebootstrapAction()
	case "mmdebstrap":
		y.Action = NewMmdebstrapAction()
	case "pacstrap":
		y.Action = &PacstrapAction{}
	case "pack":
		y.Action = NewPackAction()
	case "unpack":
		y.Action = &UnpackAction{}
	case "run":
		y.Action = &RunAction{}
	case "apt":
		y.Action = NewAptAction()
	case "pacman":
		y.Action = &PacmanAction{}
	case "ostree-commit":
		y.Action = &OstreeCommitAction{}
	case "ostree-deploy":
		y.Action = NewOstreeDeployAction()
	case "overlay":
		y.Action = &OverlayAction{}
	case "image-partition":
		y.Action = &ImagePartitionAction{}
	case "filesystem-deploy":
		y.Action = NewFilesystemDeployAction()
	case "raw":
		y.Action = &RawAction{}
	case "download":
		y.Action = &DownloadAction{}
	case "recipe":
		y.Action = &RecipeAction{}
	default:
		return fmt.Errorf("unknown action: %v", aux.Action)
	}

	err = unmarshal(y.Action)
	if err != nil {
		return err
	}

	return nil
}

// Deprecated we don't know the sector size when processing the template,
// the sector size can be defined using yaml
// definition. Append a 's' suffix and let the raw
// action to calculate the correct value.
func sector(s int) string {
	return strconv.Itoa(s) + "s"
}

func escape(s string) string {
	return shellescape.Quote(s)
}

func uuid5(namespace string, data string) string {
	return debos.GenerateUUID5(namespace, data)
}

func DumpActionStruct(iface interface{}) string {
	var a []string

	s := reflect.ValueOf(iface)
	t := reflect.TypeOf(iface)

	for i := 0; i < t.NumField(); i++ {
		f := s.Field(i)
		// Dump only exported entries
		if f.CanInterface() {
			str := fmt.Sprintf("%s: %v", s.Type().Field(i).Name, f.Interface())
			a = append(a, str)
		}
	}

	return strings.Join(a, ", ")
}

const tabs = 2

func DumpActions(iface interface{}, depth int) {
	tab := strings.Repeat(" ", depth*tabs)
	entries := reflect.ValueOf(iface)

	for i := 0; i < entries.NumField(); i++ {
		if entries.Type().Field(i).Name == "Actions" {
			log.Printf("%s  %s:\n", tab, entries.Type().Field(i).Name)
			actions := reflect.ValueOf(entries.Field(i).Interface())
			for j := 0; j < actions.Len(); j++ {
				yaml := reflect.ValueOf(actions.Index(j).Interface())
				DumpActionFields(yaml.Field(0).Interface(), depth+1)
			}
		} else {
			log.Printf("%s  %s: %v\n", tab, entries.Type().Field(i).Name, entries.Field(i).Interface())
		}
	}
}

func DumpActionFields(iface interface{}, depth int) {
	tab := strings.Repeat(" ", depth*tabs)
	entries := reflect.ValueOf(iface).Elem()

	for i := 0; i < entries.NumField(); i++ {
		f := entries.Field(i)
		// Dump only exported entries
		if f.CanInterface() {
			switch f.Kind() {
			case reflect.Struct:
				if entries.Type().Field(i).Type.String() == "debos.BaseAction" {
					// BaseAction is the only struct embbed in Action ActionFields
					// dump it at the same level
					log.Printf("%s- %s", tab, DumpActionStruct(f.Interface()))
				}

			case reflect.Slice:
				s := reflect.ValueOf(f.Interface())
				if s.Len() > 0 && s.Index(0).Kind() == reflect.Struct {
					log.Printf("%s  %s:\n", tab, entries.Type().Field(i).Name)
					for j := 0; j < s.Len(); j++ {
						if s.Index(j).Kind() == reflect.Struct {
							log.Printf("%s    { %s }", tab, DumpActionStruct(s.Index(j).Interface()))
						}
					}
				} else {
					log.Printf("%s  %s: %s\n", tab, entries.Type().Field(i).Name, f)
				}

			default:
				log.Printf("%s  %s: %v\n", tab, entries.Type().Field(i).Name, f.Interface())
			}
		}
	}
}

/*
Parse method reads YAML recipe file and map all steps to appropriate actions.

- file -- is the path to configuration file

- templateVars -- optional argument allowing to use custom map for templating
engine. Multiple template maps have no effect; only first map will be used.
*/
func (r *Recipe) Parse(file string, printRecipe bool, dump bool, templateVars ...map[string]string) error {
	t := template.New(path.Base(file))
	funcs := template.FuncMap{
		"sector": sector,
		"escape": escape,
		"uuid5":  uuid5,
	}
	t.Funcs(funcs)

	/* Add slim-sprig functions to template language */
	t.Funcs(sprig.FuncMap())

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

	if printRecipe || dump {
		log.Printf("Template variables:")
		for k, v := range templateVars[0] {
			log.Printf("\t%s:%s", k, v)
		}

		log.Printf("Recipe '%s':", file)
	}

	if printRecipe {
		log.Printf("%s", data)
	}

	if err := yaml.Unmarshal(data.Bytes(), r); err != nil {
		return err
	}

	if dump {
		DumpActions(reflect.ValueOf(*r).Interface(), 0)
	}

	if len(r.Architecture) == 0 {
		return fmt.Errorf("Recipe file must have 'architecture' property")
	}

	if len(r.Actions) == 0 {
		return fmt.Errorf("Recipe file must have at least one action")
	}

	if r.SectorSize == 0 {
		r.SectorSize = 512
	}

	return nil
}
