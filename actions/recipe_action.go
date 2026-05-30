/*
Recipe Action

This action includes the recipe at the given path, and can optionally
override or set template variables.

To ensure compatibility, both the parent recipe and all included recipes have
to be for the same architecture. For convenience the parent architecture is
passed in the "architecture" template variable.

Limitations of combined recipes are equivalent to limitations within a
single recipe (e.g. there can only be one image partition action).

	# Yaml syntax:
	- action: recipe
	  recipe: path to recipe
	  variables:
	    key: value

Mandatory properties:

- recipe -- includes the recipe actions at the given path.

Optional properties:

- variables -- overrides or adds new template variables.
*/
package actions

import (
	"errors"
	"fmt"
	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type RecipeAction struct {
	debos.BaseAction `yaml:",inline"`
	Recipe           string
	Variables        map[string]string
	Actions          Recipe `yaml:"-"`
	templateVars     map[string]string
	context          debos.Context
}

func (recipe *RecipeAction) Verify(context *debos.Context) error {
	if len(recipe.Recipe) == 0 {
		return errors.New("'recipe' property can't be empty")
	}

	recipe.context = *context

	file := recipe.Recipe
	if !filepath.IsAbs(file) {
		file = filepath.Clean(context.RecipeDir + "/" + recipe.Recipe)
	}
	recipe.context.RecipeDir = filepath.Dir(file)

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return err
	}

	// Initialise template vars
	recipe.templateVars = make(map[string]string)
	recipe.templateVars["architecture"] = context.Architecture

	// Add Variables to template vars
	for k, v := range recipe.Variables {
		recipe.templateVars[k] = v
	}

	if err := recipe.Actions.Parse(file, context.PrintRecipe, context.Verbose, recipe.templateVars); err != nil {
		// TODO: possibly do this in the caller?
		// err contains multiple lines - log them individually to retain timestamp
		log.Println("Recipe parsing failed:")
		for _, line := range strings.Split(strings.TrimRight(err.Error(), "\n"), "\n") {
			log.Printf("%s", line)
		}

		return fmt.Errorf("recipe parsing failed")
	}

	if recipe.context.Architecture != recipe.Actions.Architecture {
		return fmt.Errorf("expected architecture '%s' but got '%s'", context.Architecture, recipe.Actions.Architecture)
	}

	for _, a := range recipe.Actions.Actions {
		if err := a.Verify(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreMachine(_ *debos.Context, m *fakemachine.Machine, args *[]string) error {
	// TODO: check args?

	m.AddVolume(recipe.context.RecipeDir)

	for _, a := range recipe.Actions.Actions {
		if err := a.PreMachine(&recipe.context, m, args); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreNoMachine(_ *debos.Context) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PreNoMachine(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Run(_ *debos.Context) error {
	for _, a := range recipe.Actions.Actions {
		log.Printf("==== %s ====\n", a)
		if err := a.Run(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Cleanup(_ *debos.Context) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.Cleanup(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachine(_ *debos.Context) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachine(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachineCleanup(_ *debos.Context) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachineCleanup(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}
