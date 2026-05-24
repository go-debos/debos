/*
Template Action

Render a text template from the recipe directory into the target filesystem.
The template is processed using Go's standard text/template package.

	# Yaml syntax:
	- action: template
	  source: path/to/template.in
	  destination: /absolute/path/inside/rootfs
	  variables:
	    key1: value1
	    key2: value2

Mandatory properties:

- source -- path to the template file, relative to the recipe directory.

- destination -- absolute path inside the target rootfs where the rendered
file will be written. Any missing parent directories will be created.

Optional properties:

- variables -- map of string keys to values passed to the Go template
renderer. Values are referenced from the template as `{{ .key }}`.

The rendered file inherits the mode of the source template, matching the
behaviour of the overlay action. Referencing an undefined variable in the
template produces an error.

Example: render `/etc/os-release` from a template file shipped alongside
the recipe:

  - action: template
    source: files/os-release.in
    destination: /etc/os-release
    variables:
    image_name: "Example OS"
    image_version: "1.0"
    variant: "${variant}"

With `files/os-release.in` containing:

	NAME="{{ .image_name }}"
	VERSION="{{ .image_version }}"
	VARIANT="{{ .variant }}"
*/
package actions

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"text/template"

	"github.com/go-debos/debos"
)

type TemplateAction struct {
	debos.BaseAction `yaml:",inline"`
	Source           string            // path to the template file, relative to recipe dir
	Destination      string            // absolute path inside the target rootfs
	Variables        map[string]string // variables passed to the template engine
}

func (t *TemplateAction) Verify(context *debos.Context) error {
	if len(t.Source) == 0 {
		return fmt.Errorf("property 'source' is mandatory for template action")
	}
	if len(t.Destination) == 0 {
		return fmt.Errorf("property 'destination' is mandatory for template action")
	}

	if _, err := debos.RestrictedPath(context.Rootdir, t.Destination); err != nil {
		return err
	}

	source := debos.CleanPathAt(t.Source, context.RecipeDir)
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("template source '%s': %w", t.Source, err)
	}

	return nil
}

func (t *TemplateAction) Run(context *debos.Context) error {
	source := debos.CleanPathAt(t.Source, context.RecipeDir)

	destination, err := debos.RestrictedPath(context.Rootdir, t.Destination)
	if err != nil {
		return err
	}

	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("template source '%s': %w", t.Source, err)
	}

	tmpl, err := template.New(path.Base(source)).Option("missingkey=error").ParseFiles(source)
	if err != nil {
		return fmt.Errorf("failed to parse template '%s': %w", t.Source, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, t.Variables); err != nil {
		return fmt.Errorf("failed to render template '%s': %w", t.Source, err)
	}

	destinationParent := path.Dir(destination)
	if err := os.MkdirAll(destinationParent, 0755); err != nil {
		return fmt.Errorf("could not create parent destination path '%s': %w", destinationParent, err)
	}

	log.Printf("Rendering template %s to %s", source, destination)
	if err := os.WriteFile(destination, rendered.Bytes(), info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to write rendered template to '%s': %w", destination, err)
	}

	return nil
}
