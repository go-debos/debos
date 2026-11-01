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

Variables passed via the `-t NAME:VALUE` command line argument are available as
a field on the template data object, not as local variables:

	# Use a variable passed with `-t Var:Value`
	property: {{ .Var }}

This is distinct from local variables declared inside the recipe itself,
which use the `$Var` syntax:

	{{- $Var := "Value" -}}
	property: {{ $Var }}

The following custom template functions are available:

- sector: Returns the argument with 's' suffix for raw action` (Deprecated)
- escape: Shell escape the  argument `{{ escape $var }}`
- uuid5: Generates fixed UUID value `{{ uuid5 $random-uuid $text }}`
- functions from [slim-sprig](https://go-task.github.io/slim-sprig/)

Mandatory properties for recipe:

- architecture -- target architecture

- actions -- at least one action should be listed

Optional properties for recipe:

- sectorsize: Overrides the default 512 bytes sectorsize, mandatory for device using 4k block size such as UFS or NVMe storage.

Unknown properties, in the header or in any action, are reported with a
warning when the recipe is parsed, so a misspelled or unsupported property is
not silently ignored. When debos is run with --strict, unknown properties are
an error and the build fails early. Properties prefixed with '.' are exempt
from this check and are ignored, which allows defining reusable YAML anchors:

	.common-run-settings: &common-run-settings
	  chroot: false

	actions:
	  - action: run
	    <<: *common-run-settings
	    command: echo hello

# Supported actions

- apt -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Apt_Action

- debootstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Debootstrap_Action

- mmdebstrap -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Mmdebstrap_Action

- download -- https://godoc.org/github.com/go-debos/debos/actions#hdr-Download_Action

- filesystem-deploy -- https://godoc.org/github.com/go-debos/debos/actions#hdr-FilesystemDeploy_Action

- image-partition -- https://godoc.org/github.com/go-debos/debos/actions#hdr-ImagePartition_Action

- install-deb -- https://godoc.org/github.com/go-debos/debos/actions#hdr-InstallDeb_Action

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
	"github.com/go-debos/fakemachine"
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
	var raw map[string]interface{}

	if err := unmarshal(&raw); err != nil {
		return err
	}

	action, ok := raw["action"].(string)
	if !ok || action == "" {
		return fmt.Errorf("missing or invalid 'action' field")
	}

	switch action {
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
	case "install-deb":
		y.Action = NewInstallDebAction()
	case "filesystem-deploy":
		y.Action = NewFilesystemDeployAction()
	case "raw":
		y.Action = &RawAction{}
	case "download":
		y.Action = &DownloadAction{}
	case "recipe":
		y.Action = &RecipeAction{}
	default:
		return fmt.Errorf("unknown action: %v", action)
	}

	err := unmarshal(y.Action)
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

func uuid5(namespace, data string) (string, error) {
	uuid, err := debos.GenerateUUID5(namespace, data)
	return uuid.String(), err
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

/* logMultiline logs msg followed by err. The error may span multiple lines
 * (e.g. a YAML source snippet pointing at the problem), so log each line
 * individually to retain the timestamp prefix. */
func logMultiline(msg string, err error) {
	log.Printf("%s", msg)
	for _, line := range strings.Split(strings.TrimRight(err.Error(), "\n"), "\n") {
		log.Printf("%s", line)
	}
}

/* logWarning logs a recipe validation warning. Recipes are parsed on the
 * host and again inside fakemachine, so only warn on the host to avoid
 * printing each warning twice. */
func logWarning(msg string, err error) {
	if fakemachine.InMachine() {
		return
	}
	logMultiline(msg, err)
}

/* checkPostprocessOrder ensures all run actions with postprocess set are at
 * the end of the list of actions. */
func checkPostprocessOrder(actions []YamlAction) error {
	var firstPostprocess int
	seenPostprocess := false
	for i, action := range actions {
		runAction, isRunAction := action.Action.(*RunAction)
		isPostprocess := isRunAction && runAction.PostProcess

		if !seenPostprocess {
			if isPostprocess {
				firstPostprocess = i
				seenPostprocess = true
			}
			continue
		}

		if !isPostprocess {
			return fmt.Errorf("action %d (%s) is listed after postprocess run action %d (%s), but postprocess run actions are executed after all other actions; move postprocess run actions to the end of the recipe",
				i+1, action.String(), firstPostprocess+1, actions[firstPostprocess].String())
		}
	}
	return nil
}

/* LogParseError logs an error returned by Recipe.Parse. */
func LogParseError(file string, err error) {
	logMultiline(fmt.Sprintf("Recipe '%s' parsing failed:", file), err)
}

/*
Parse method reads YAML recipe file and map all steps to appropriate actions.

- file -- is the path to configuration file

- strict -- treat recipe validation warnings as errors

- templateVars -- optional argument allowing to use custom map for templating
engine. Multiple template maps have no effect; only first map will be used.
*/
func (r *Recipe) Parse(file string, printRecipe bool, dump bool, strict bool, templateVars ...map[string]string) error {
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

	// Disallow unknown fields (e.g. those which are not actually available in an action)
	// but allow fields prefixed with "." at any level, which is useful for YAML anchors.
	if err := yaml.UnmarshalWithOptions(
		data.Bytes(),
		r,
		yaml.DisallowUnknownField(),
		yaml.AllowFieldPrefixes("."),
	); err != nil {
		if strict {
			return err
		}

		// Retry allowing unknown fields; if that succeeds then unknown
		// fields were the only problem, so only warn about them.
		*r = Recipe{}
		if err := yaml.Unmarshal(data.Bytes(), r); err != nil {
			return err
		}
		logWarning(fmt.Sprintf("WARNING: Recipe '%s' has unknown properties (use --strict to make this an error):", file), err)
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

	/* Postprocess run actions are executed on the host after all other actions
	 * (including those in child recipes) have run, so any other action listed
	 * after one is misleading: it will actually run before it. */
	if err := checkPostprocessOrder(r.Actions); err != nil {
		if strict {
			return err
		}
		logWarning(fmt.Sprintf("WARNING: Recipe '%s' has misordered actions (use --strict to make this an error):", file), err)
	}

	if r.SectorSize == 0 {
		r.SectorSize = 512
	}

	return nil
}
