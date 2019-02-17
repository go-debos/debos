package actions_test

import (
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
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
		err = r.Parse(test.filename, false)
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
		err = r.Parse(file.Name(), false)
	} else {
		err = r.Parse(file.Name(), false, templateVars[0])
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
