package actions_test

import (
	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
	"strings"
)

type testRecipe struct {
	recipe string
	err    string
}

// Test if incorrect file has been passed
func TestParse_incorrect_file(t *testing.T) {
	var err error

	var tests = []struct {
		filename string
		err      string
	}{
		{
			"non-existing.yaml",
			"open non-existing.yaml: no such file or directory",
		},
		{
			"/proc",
			"read /proc: is a directory",
		},
	}

	for _, test := range tests {
		r := actions.Recipe{}
		err = r.Parse(test.filename, false, false)
		assert.EqualError(t, err, test.err)
	}
}

// Check common recipe syntax
func TestParse_syntax(t *testing.T) {

	var tests = []testRecipe{
		// Test if all actions are supported
		{`
architecture: arm64

actions:
  - action: apt-file
  - action: apt
  - action: debootstrap
  - action: download
  - action: filesystem-deploy
  - action: image-partition
  - action: ostree-commit
  - action: ostree-deploy
  - action: overlay
  - action: pack
  - action: raw
  - action: run
  - action: unpack
  - action: recipe
`,
			"", // Do not expect failure
		},
		// Test of unknown action in list
		{`
architecture: arm64

actions:
  - action: test_unknown_action
`,
			"Unknown action: test_unknown_action",
		},
		// Test if 'architecture' property absence
		{`
actions:
  - action: raw
`,
			"Recipe file must have 'architecture' property",
		},
		// Test if no actions listed
		{`
architecture: arm64
`,
			"Recipe file must have at least one action",
		},
		// Test of wrong syntax in Yaml
		{`wrong`,
			"yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `wrong` into actions.Recipe",
		},
		// Test if no actions listed
		{`
architecture: arm64
`,
			"Recipe file must have at least one action",
		},
	}

	for _, test := range tests {
		runTest(t, test)
	}
}

// Check template engine
func TestParse_template(t *testing.T) {

	var test = testRecipe{
		// Test template variables
		`
{{ $action:= or .action "download" }}
architecture: arm64
actions:
  - action: {{ $action }}
`,
		"", // Do not expect failure
	}

	{ // Test of embedded template
		r := runTest(t, test)
		assert.Equalf(t, r.Actions[0].String(), "download",
			"Fail to use embedded variable definition from recipe:%s\n",
			test.recipe)
	}

	{ // Test of user-defined template variable
		var templateVars = map[string]string{
			"action": "pack",
		}

		r := runTest(t, test, templateVars)
		assert.Equalf(t, r.Actions[0].String(), "pack",
			"Fail to redefine variable with user-defined map:%s\n",
			test.recipe)
	}
}

// Test of 'sector' function embedded to recipe package
func TestParse_sector(t *testing.T) {
	var testSector = testRecipe{
		// Fail with unknown action
		`
architecture: arm64

actions:
  - action: {{ sector 42 }}
`,
		"Unknown action: 21504",
	}
	runTest(t, testSector)
}

func runTest(t *testing.T, test testRecipe, templateVars ...map[string]string) actions.Recipe {
	file, err := ioutil.TempFile(os.TempDir(), "recipe")
	assert.Empty(t, err)
	defer os.Remove(file.Name())

	file.WriteString(test.recipe)
	file.Close()

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	failed := false

	if len(test.err) > 0 {
		// Expected error?
		failed = !assert.EqualError(t, err, test.err)
	} else {
		// Unexpected error
		failed = !assert.Empty(t, err)
	}

	if failed {
		t.Logf("Failed recipe:%s\n", test.recipe)
	}

	return r
}

type subRecipe struct {
	name string
	recipe string
}

type testSubRecipe struct {
	recipe string
	subrecipe subRecipe
	err    string
}

func TestSubRecipe(t *testing.T) {
	// Embedded recipes
	var recipeAmd64 = subRecipe {
		"amd64.yaml",
		`
architecture: amd64

actions:
  - action: run
    command: ok.sh
`,
	}
	var recipeInheritedArch = subRecipe {
		"inherited.yaml",
		`
{{- $architecture := or .architecture "armhf" }}
architecture: {{ $architecture }}

actions:
  - action: run
    command: ok.sh
`,
	}
	var recipeArmhf = subRecipe {
		"armhf.yaml",
		`
architecture: armhf

actions:
  - action: run
    command: ok.sh
`,
	}

	// test recipes
	var tests = []testSubRecipe {
		{
		// Test recipe same architecture OK
		`
architecture: amd64

actions:
  - action: recipe
    recipe: amd64.yaml
`,
		recipeAmd64,
		"", // Do not expect failure
		},
		{
		// Test recipe with inherited architecture OK
		`
architecture: amd64

actions:
  - action: recipe
    recipe: inherited.yaml
`,
		recipeInheritedArch,
		"", // Do not expect failure
		},
		{
		// Fail with unknown recipe
		`
architecture: amd64

actions:
  - action: recipe
    recipe: unknown_recipe.yaml
`,
		recipeAmd64,
		"stat /tmp/unknown_recipe.yaml: no such file or directory",
		},
		{
		// Fail with different architecture recipe
		`
architecture: amd64

actions:
  - action: recipe
    recipe: armhf.yaml
`,
		recipeArmhf,
		"Expect architecture 'amd64' but got 'armhf'",
		},
	}

	for _, test := range tests {
		runTestWithSubRecipes(t, test)
	}
}

func runTestWithSubRecipes(t *testing.T, test testSubRecipe, templateVars ...map[string]string) actions.Recipe {
	context := debos.DebosContext { &debos.CommonContext{}, "", "" }
	dir, err := ioutil.TempDir("", "go-debos")
	assert.Empty(t, err)
	defer os.RemoveAll(dir)

	file, err := ioutil.TempFile(dir, "recipe")
	assert.Empty(t, err)
	defer os.Remove(file.Name())

	file.WriteString(test.recipe)
	file.Close()

	file_subrecipe, err := os.Create(dir + "/" + test.subrecipe.name)
	assert.Empty(t, err)
	defer os.Remove(file_subrecipe.Name())

	file_subrecipe.WriteString(test.subrecipe.recipe)
	file_subrecipe.Close()

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	// Should not expect error during parse
	failed := !assert.Empty(t, err)

	if !failed {
		context.Architecture = r.Architecture
		context.RecipeDir = dir

		for _, a := range r.Actions {
			if err = a.Verify(&context); err != nil {
				break
			}
		}

		if len(test.err) > 0 {
			// Expected error?
			failed = !assert.EqualError(t, err, strings.Replace(test.err, "/tmp", dir, 1))
		} else {
			// Unexpected error
			failed = !assert.Empty(t, err)
		}
	}

	if failed {
		t.Logf("Failed recipe:%s\n", test.recipe)
	}

	return r
}
