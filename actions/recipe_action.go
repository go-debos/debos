/*
Recipe Action

Include a recipe.

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
	"os"
	"path/filepath"
	"github.com/go-debos/debos"
	"github.com/go-debos/fakemachine"
)

type RecipeAction struct {
	debos.BaseAction `yaml:",inline"`
	Recipe           string
	Variables        map[string]string
	Actions          Recipe `yaml:"-"`
	templateVars     map[string]string
}

func (recipe *RecipeAction) Verify(context *debos.DebosContext) error {
	if len(recipe.Recipe) == 0 {
		return errors.New("'recipe' property can't be empty")
	}

	file := recipe.Recipe
	if !filepath.IsAbs(file) {
		file = filepath.Clean(context.RecipeDir + "/" + recipe.Recipe)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return err
	}

	// Initialise template vars
	recipe.templateVars = make(map[string]string)
	recipe.templateVars["included_recipe"] = "true"
	recipe.templateVars["architecture"] = context.Architecture

	// Add Variables to template vars
	for k, v := range recipe.Variables {
		recipe.templateVars[k] = v
	}

	if err := recipe.Actions.Parse(file, context.PrintRecipe, context.Verbose, recipe.templateVars); err != nil {
		return err
	}

	if context.Architecture != recipe.Actions.Architecture {
		return fmt.Errorf("Expect architecture '%s' but got '%s'", context.Architecture, recipe.Actions.Architecture)
	}

	for _, a := range recipe.Actions.Actions {
		if err := a.Verify(context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreMachine(context *debos.DebosContext, m *fakemachine.Machine, args *[]string) error {
	// TODO: check args?

	for _, a := range recipe.Actions.Actions {
		if err := a.PreMachine(context, m, args); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PreNoMachine(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PreNoMachine(context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Run(context *debos.DebosContext) error {
	recipe.LogStart()

	for _, a := range recipe.Actions.Actions {
		if err := a.Run(context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) Cleanup(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.Cleanup(context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachine(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachine(context); err != nil {
			return err
		}
	}

	return nil
}

func (recipe *RecipeAction) PostMachineCleanup(context *debos.DebosContext) error {
	for _, a := range recipe.Actions.Actions {
		if err := a.PostMachineCleanup(context); err != nil {
			return err
		}
	}

	return nil
}
