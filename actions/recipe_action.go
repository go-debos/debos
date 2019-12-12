/*
Recipe Action

This action includes the recipe at the given path, and can optionally
override or set template variables.

To ensure compatibility, both the parent recipe and all included recipes have
to be for the same architecture. For convenience the parent architecture is
passed in the "architecture" template variable.

Limitations of combined recipes are equivalent to limitations within a
single recipe (e.g. there can only be one image partition action).

Yaml syntax:
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
	"os"
	"path/filepath"
)

type RecipeAction struct {
	debos.BaseAction `yaml:",inline"`
	Recipe           string
	Variables        map[string]string
	Actions          Recipe `yaml:"-"`
	templateVars     map[string]string
	context          debos.DebosContext
}

func (recipe *RecipeAction) Verify(context *debos.DebosContext) error {
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
		return err
	}

	if recipe.context.Architecture != recipe.Actions.Architecture {
		return fmt.Errorf("Expect architecture '%s' but got '%s'", context.Architecture, recipe.Actions.Architecture)
	}

	for _, a := range recipe.Actions.Actions {
		if err := a.Verify(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine, args *[]string) error {
	// TODO: check args?

	m.AddVolume(recipe.context.RecipeDir)

	for _, a := range recipe.Actions.Actions {
		if err := a.PreMachine(&recipe.context, m, args); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreNoMachine(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PreNoMachine(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Run(context *debos.DebosContext) error {
	recipe.LogStart()

	for _, a := range recipe.Actions.Actions {
		if err := a.Run(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Cleanup(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.Cleanup(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachine(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachine(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachineCleanup(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachineCleanup(&recipe.context); err != nil {
			return err
		}
	}

	return nil
}
