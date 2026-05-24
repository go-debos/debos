// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

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

- destination -- absolute path of the file inside the target rootfs where the
rendered template will be written. Any missing parent directories will be
created.

Optional properties:

- variables -- map of string keys to values passed to the Go template
renderer. Values are referenced from the template as `{{ .key }}`.

The same template functions as in recipes are available, see the recipe
action documentation.

The template is parsed and rendered while the recipe is verified, so syntax
errors and references to undefined variables are reported before the build
starts.

The rendered file is given the mode of the source template. An existing
destination is replaced; if it is a symlink, the symlink itself is replaced
rather than the file it points to.

Example: render `/etc/os-release` from a template file shipped alongside
the recipe, passing a recipe variable through to the template:

	# recipe.yaml
	- action: template
	  source: files/os-release.in
	  destination: /etc/os-release
	  variables:
	    image_name: "Example OS"
	    image_version: "1.0"
	    variant: {{ $variant }}

With `files/os-release.in` containing:

	NAME="{{ .image_name }}"
	VERSION="{{ .image_version }}"
	VARIANT="{{ .variant }}"
	ID="{{ .image_name | lower | replace " " "-" }}"
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

// render parses and executes the source template, returning the rendered
// content and the mode of the source file.
func (t *TemplateAction) render(context *debos.Context) (*bytes.Buffer, os.FileMode, error) {
	source := debos.CleanPathAt(t.Source, context.RecipeDir)

	info, err := os.Stat(source)
	if err != nil {
		return nil, 0, fmt.Errorf("template source '%s': %w", t.Source, err)
	}

	tmpl := addTemplateFuncs(template.New(path.Base(source)).Option("missingkey=error"))
	if _, err := tmpl.ParseFiles(source); err != nil {
		return nil, 0, fmt.Errorf("failed to parse template '%s': %w", t.Source, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, t.Variables); err != nil {
		return nil, 0, fmt.Errorf("failed to render template '%s': %w", t.Source, err)
	}

	return &rendered, info.Mode().Perm(), nil
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

	_, _, err := t.render(context)
	return err
}

func (t *TemplateAction) Run(context *debos.Context) error {
	destination, err := debos.RestrictedPath(context.Rootdir, t.Destination)
	if err != nil {
		return err
	}

	rendered, mode, err := t.render(context)
	if err != nil {
		return err
	}

	destinationParent := path.Dir(destination)
	if err := os.MkdirAll(destinationParent, 0755); err != nil {
		return fmt.Errorf("could not create parent destination path '%s': %w", destinationParent, err)
	}

	log.Printf("Rendering template %s to %s", t.Source, destination)
	if err := debos.WriteFile(destination, rendered, mode); err != nil {
		return fmt.Errorf("failed to write rendered template to '%s': %w", destination, err)
	}

	return nil
}
