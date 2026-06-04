package actions_test

import (
	"os"
	"strings"
	"testing"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRecipe struct {
	recipe string
	err    string
}

// Test if incorrect file has been passed
func TestParse_incorrect_file(t *testing.T) {
	var err error

	tests := []struct {
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
		if test.err != "" {
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.err)
		} else {
			assert.NoError(t, err)
		}
	}
}

// Check common recipe syntax
func TestParse_syntax(t *testing.T) {
	tests := []testRecipe{
		// Test if all actions are supported
		{
			`
architecture: arm64

actions:
  - action: apt
  - action: debootstrap
  - action: download
  - action: filesystem-deploy
  - action: image-partition
  - action: install-deb
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
		{
			`
architecture: arm64

actions:
  - action: test_unknown_action
`,
			"unknown action: test_unknown_action",
		},
		// Test if 'architecture' property absence
		{
			`
actions:
  - action: raw
`,
			"Recipe file must have 'architecture' property",
		},
		// Test if no actions listed
		{
			`
architecture: arm64
`,
			"Recipe file must have at least one action",
		},
		// Test of wrong syntax in Yaml
		{
			`wrong`,
			"[1:1] string was used where mapping is expected\n>  1 | wrong\n       ^\n",
		},
		// Test if no actions listed
		{
			`
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
	test := testRecipe{
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
		assert.Equalf(t, "download", r.Actions[0].String(),
			"Fail to use embedded variable definition from recipe:%s\n",
			test.recipe)
	}

	{ // Test of user-defined template variable
		templateVars := map[string]string{
			"action": "pack",
		}

		r := runTest(t, test, templateVars)
		assert.Equalf(t, "pack", r.Actions[0].String(),
			"Fail to redefine variable with user-defined map:%s\n",
			test.recipe)
	}
}

// Test of 'sector' function embedded to recipe package
func TestParse_sector(t *testing.T) {
	testSector := testRecipe{
		// Fail with unknown action
		`
architecture: arm64

actions:
  - action: {{ sector 42 }}
`,
		"unknown action: 42s",
	}
	runTest(t, testSector)
}

func runTest(t *testing.T, test testRecipe, templateVars ...map[string]string) actions.Recipe {
	file, err := os.CreateTemp(os.TempDir(), "recipe")
	require.NoError(t, err)
	defer func() { _ = os.Remove(file.Name()) }()

	_, _ = file.WriteString(test.recipe)
	_ = file.Close()

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Failed recipe:%s\n", test.recipe)
		}
	})

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	if len(test.err) > 0 {
		// Expected error?
		require.ErrorContains(t, err, test.err)
	} else {
		// Unexpected error
		require.NoError(t, err)
	}

	return r
}

type subRecipe struct {
	name   string
	recipe string
}

type testSubRecipe struct {
	recipe    string
	subrecipe subRecipe
	err       string
	parseErr  string
}

func TestSubRecipe(t *testing.T) {
	// Embedded recipes
	recipeAmd64 := subRecipe{
		"amd64.yaml",
		`
architecture: amd64

actions:
  - action: run
    command: ok.sh
`,
	}
	recipeInheritedArch := subRecipe{
		"inherited.yaml",
		`
{{- $architecture := or .architecture "armhf" }}
architecture: {{ $architecture }}

actions:
  - action: run
    command: ok.sh
`,
	}
	recipeArmhf := subRecipe{
		"armhf.yaml",
		`
architecture: armhf

actions:
  - action: run
    command: ok.sh
`,
	}

	// test recipes
	tests := []testSubRecipe{
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
			"", // Do not expect parse failure
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
			"", // Do not expect parse failure
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
			"", // Do not expect parse failure
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
			"expected architecture 'amd64' but got 'armhf'",
			"", // Do not expect parse failure
		},
		{
			// Fail with type mismatch during parse
			`
architecture: armhf

actions:
  - action: recipe
    recipe: armhf.yaml
    variables:
      - foo
`,
			recipeArmhf,
			"",
			"[8:7] sequence was used where mapping is expected\n   5 |   - action: recipe\n   6 |     recipe: armhf.yaml\n   7 |     variables:\n>  8 |       - foo\n             ^\n",
		},
	}

	for _, test := range tests {
		runTestWithSubRecipes(t, test)
	}
}

func runTestWithSubRecipes(t *testing.T, test testSubRecipe, templateVars ...map[string]string) actions.Recipe {
	context := debos.Context{
		CommonContext: &debos.CommonContext{},
		RecipeDir:     "",
		Architecture:  "",
		SectorSize:    512,
	}
	dir, err := os.MkdirTemp("", "go-debos")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	file, err := os.CreateTemp(dir, "recipe")
	require.NoError(t, err)
	defer func() { _ = os.Remove(file.Name()) }()

	_, _ = file.WriteString(test.recipe)
	_ = file.Close()

	fileSubrecipe, err := os.Create(dir + "/" + test.subrecipe.name)
	require.NoError(t, err)
	defer func() { _ = os.Remove(fileSubrecipe.Name()) }()

	_, _ = fileSubrecipe.WriteString(test.subrecipe.recipe)
	_ = fileSubrecipe.Close()

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Failed recipe:%s\n", test.recipe)
		}
	})

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	if len(test.parseErr) > 0 {
		// Expected parse error?
		require.ErrorContains(t, err, test.parseErr)
	} else {
		// Unexpected error
		require.NoError(t, err)
	}

	if err == nil {
		context.Architecture = r.Architecture
		context.SectorSize = r.SectorSize
		context.RecipeDir = dir

		for _, a := range r.Actions {
			if err = a.Verify(&context); err != nil {
				break
			}
		}

		if len(test.err) > 0 {
			// Expected error?
			expected := strings.Replace(test.err, "/tmp", dir, 1)
			require.ErrorContains(t, err, expected)
		} else {
			// Unexpected error
			require.NoError(t, err)
		}
	}

	return r
}
