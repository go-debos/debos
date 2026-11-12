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

// Test if incorrect file has been passed.
func TestParse_incorrect_file(t *testing.T) {
	var err error

	tests := []struct {
		filename string
		err      string
	}{
		{
			"non-existing.yaml",
			"failed to parse non-existing.yaml: open non-existing.yaml: no such file or directory",
		},
		{
			"/proc",
			"failed to parse /proc: read /proc: is a directory",
		},
	}

	for _, test := range tests {
		r := actions.Recipe{}
		err = r.Parse(test.filename, false, false)
		assert.EqualError(t, err, test.err)
	}
}

// Check common recipe syntax.
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
			"failed to unmarshal {file}: unknown action: test_unknown_action",
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
			"failed to unmarshal {file}: [1:1] string was used where mapping is expected\n>  1 | wrong\n       ^\n",
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

// Check template engine.
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

// Test of 'sector' function embedded to recipe package.
func TestParse_sector(t *testing.T) {
	testSector := testRecipe{
		// Fail with unknown action
		`
architecture: arm64

actions:
  - action: {{ sector 42 }}
`,
		"failed to unmarshal {file}: unknown action: 42s",
	}
	runTest(t, testSector)
}

func runTest(t *testing.T, test testRecipe, templateVars ...map[string]string) actions.Recipe {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "recipe")
	require.NoError(t, err)

	_, err = file.WriteString(test.recipe)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	defer func() {
		if t.Failed() {
			t.Logf("Failed recipe: %s", test.recipe)
		}
	}()

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	if len(test.err) > 0 {
		// Expected error?
		require.EqualError(t, err, strings.ReplaceAll(test.err, "{file}", file.Name()))
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
			"failed to find recipe: stat /tmp/unknown_recipe.yaml: no such file or directory",
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
			"failed to unmarshal {file}: [8:7] sequence was used where mapping is expected\n   5 |   - action: recipe\n   6 |     recipe: armhf.yaml\n   7 |     variables:\n>  8 |       - foo\n             ^\n",
		},
	}

	for _, test := range tests {
		runTestWithSubRecipes(t, test)
	}
}

func TestSingleImagePartition(t *testing.T) {
	const imagePartition = `
  - action: image-partition
    imagename: test.img
    imagesize: 1G
    partitiontype: gpt
    mountpoints:
      - mountpoint: /
        partition: root
    partitions:
      - name: root
        fs: ext4
        start: 1M
        end: 100%
`
	const errMultiple = "only one image-partition action is supported"
	const errMultipleSub = "failed to verify action (image-partition): " + errMultiple

	recipeImage := subRecipe{
		"image.yaml",
		"architecture: amd64\nactions:" + imagePartition,
	}

	tests := []testSubRecipe{
		{
			// Single image-partition in a sub-recipe OK
			"architecture: amd64\nactions:\n  - action: recipe\n    recipe: image.yaml\n",
			recipeImage,
			"",
			"",
		},
		{
			// Fail with multiple image-partition actions
			"architecture: amd64\nactions:" + imagePartition + imagePartition,
			recipeImage,
			errMultiple,
			"",
		},
		{
			// Fail with image-partition before a sub-recipe with another
			"architecture: amd64\nactions:" + imagePartition + "  - action: recipe\n    recipe: image.yaml\n",
			recipeImage,
			errMultipleSub,
			"",
		},
		{
			// Fail with image-partition after a sub-recipe with another
			"architecture: amd64\nactions:\n  - action: recipe\n    recipe: image.yaml\n" + imagePartition,
			recipeImage,
			errMultiple,
			"",
		},
	}

	for _, test := range tests {
		runTestWithSubRecipes(t, test)
	}
}

func TestImageExtractNeedsImagePartition(t *testing.T) {
	const imagePartition = `
  - action: image-partition
    imagename: test.img
    imagesize: 1G
    partitiontype: gpt
    mountpoints:
      - mountpoint: /
        partition: root
    partitions:
      - name: root
        fs: ext4
        start: 1M
        end: 100%
`
	const imageExtract = "\n  - action: image-extract\n"
	const recipeAction = "  - action: recipe\n    recipe: sub.yaml\n"
	const errMissing = "image-extract must come after an image-partition action"
	const errMissingSub = "failed to verify action (image-extract): " + errMissing

	recipeImage := subRecipe{"sub.yaml", "architecture: amd64\nactions:" + imagePartition}
	recipeExtract := subRecipe{"sub.yaml", "architecture: amd64\nactions:" + imageExtract}

	tests := []testSubRecipe{
		{
			// Fail without image-partition
			"architecture: amd64\nactions:" + imageExtract,
			recipeImage,
			errMissing,
			"",
		},
		{
			// Fail with image-extract before image-partition
			"architecture: amd64\nactions:" + imageExtract + imagePartition,
			recipeImage,
			errMissing,
			"",
		},
		{
			// Fail with image-extract in a sub-recipe before image-partition
			"architecture: amd64\nactions:\n" + recipeAction + imagePartition,
			recipeExtract,
			errMissingSub,
			"",
		},
		{
			// image-partition in a sub-recipe followed by image-extract OK
			"architecture: amd64\nactions:\n" + recipeAction + imageExtract,
			recipeImage,
			"",
			"",
		},
		{
			// image-partition followed by image-extract in a sub-recipe OK
			"architecture: amd64\nactions:" + imagePartition + recipeAction,
			recipeExtract,
			"",
			"",
		},
	}

	for _, test := range tests {
		runTestWithSubRecipes(t, test)
	}
}

func runTestWithSubRecipes(t *testing.T, test testSubRecipe, templateVars ...map[string]string) {
	t.Helper()

	context := debos.Context{
		CommonContext: &debos.CommonContext{},
		RecipeDir:     "",
		Architecture:  "",
		SectorSize:    512,
	}
	dir := t.TempDir()

	file, err := os.CreateTemp(dir, "recipe")
	require.NoError(t, err)

	_, err = file.WriteString(test.recipe)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	fileSubrecipe, err := os.Create(dir + "/" + test.subrecipe.name)
	require.NoError(t, err)

	_, err = fileSubrecipe.WriteString(test.subrecipe.recipe)
	require.NoError(t, err)
	require.NoError(t, fileSubrecipe.Close())

	defer func() {
		if t.Failed() {
			t.Logf("Failed recipe: %s", test.recipe)
			t.Logf("Failed subrecipe: %s", test.subrecipe.recipe)
		}
	}()

	r := actions.Recipe{}
	if len(templateVars) == 0 {
		err = r.Parse(file.Name(), false, false)
	} else {
		err = r.Parse(file.Name(), false, false, templateVars[0])
	}

	if len(test.parseErr) > 0 {
		// Expected parse error?
		require.EqualError(t, err, strings.ReplaceAll(test.parseErr, "{file}", file.Name()))
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
			require.EqualError(t, err, strings.Replace(test.err, "/tmp", dir, 1))
		} else {
			// Unexpected error
			require.NoError(t, err)
		}
	}
}
