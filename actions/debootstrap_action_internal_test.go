// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDebootstrapScripts creates a fake host debootstrap scripts directory
// containing the given scripts.
func setupDebootstrapScripts(t *testing.T, scripts ...string) string {
	t.Helper()

	dir := t.TempDir()
	for _, s := range scripts {
		require.NoError(t, os.WriteFile(filepath.Join(dir, s), nil, 0o644))
	}

	old := debootstrapScriptsDir
	debootstrapScriptsDir = dir
	t.Cleanup(func() { debootstrapScriptsDir = old })

	return dir
}

func TestDebootstrapAction_ShouldExcludeUsrIsMerged(t *testing.T) {
	tests := []struct {
		suite    string
		expected bool
	}{
		{"etch", true},
		{"buster", true},
		{"bullseye", true},
		{"Bullseye", true},
		{"bookworm", false},
		{"trixie", false},
		{"unstable", false},
		{"jammy", false},
		{"v2023", false},
	}

	for _, tt := range tests {
		t.Run(tt.suite, func(t *testing.T) {
			assert.Equal(t, tt.expected, shouldExcludeUsrIsMerged(tt.suite))
		})
	}
}

func TestDebootstrapAction_VerifyScriptSelection(t *testing.T) {
	recipeDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(recipeDir, "custom-script"), nil, 0o644))

	tests := []struct {
		name              string
		suite             string
		debootstrapSuite  string
		debootstrapScript string
		expectedScript    string // relative to the scripts dir, or absolute
		expectedSuite     string
		expectedError     string
	}{
		{
			name:           "suite with own script",
			suite:          "bookworm",
			expectedScript: "bookworm",
			expectedSuite:  "bookworm",
		},
		{
			name:           "unknown suite falls back to unstable",
			suite:          "v2023",
			expectedScript: "unstable",
			expectedSuite:  "unstable",
		},
		{
			name:             "debootstrap-suite",
			suite:            "v2023",
			debootstrapSuite: "bullseye",
			expectedScript:   "bullseye",
			expectedSuite:    "bullseye",
		},
		{
			name:             "debootstrap-suite without script",
			suite:            "v2023",
			debootstrapSuite: "nonexistent",
			expectedError:    "cannot find debootstrap script for debootstrap-suite 'nonexistent'",
		},
		{
			name:             "debootstrap-suite when suite has own script",
			suite:            "bookworm",
			debootstrapSuite: "bullseye",
			expectedError:    "suite 'bookworm' has its own debootstrap script; 'debootstrap-suite' should not be set",
		},
		{
			name:              "debootstrap-script",
			suite:             "v2023",
			debootstrapScript: "custom-script",
			expectedScript:    filepath.Join(recipeDir, "custom-script"),
			expectedSuite:     "",
		},
		{
			name:              "debootstrap-script missing",
			suite:             "v2023",
			debootstrapScript: "missing-script",
			expectedError:     "failed to stat " + filepath.Join(recipeDir, "missing-script"),
		},
		{
			name:              "both debootstrap-suite and debootstrap-script",
			suite:             "v2023",
			debootstrapSuite:  "bullseye",
			debootstrapScript: "custom-script",
			expectedError:     "can't set both 'debootstrap-suite' and 'debootstrap-script' properties",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptsDir := setupDebootstrapScripts(t, "bookworm", "bullseye", "unstable")

			d := NewDebootstrapAction()
			d.Suite = tt.suite
			d.DebootstrapSuite = tt.debootstrapSuite
			d.DebootstrapScript = tt.debootstrapScript

			err := d.Verify(&debos.Context{CommonContext: &debos.CommonContext{}, RecipeDir: recipeDir})
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)

				return
			}
			require.NoError(t, err)

			expectedScript := tt.expectedScript
			if !filepath.IsAbs(expectedScript) {
				expectedScript = filepath.Join(scriptsDir, expectedScript)
			}
			assert.Equal(t, expectedScript, d.scriptPath)
			assert.Equal(t, tt.expectedSuite, d.scriptSuite)
		})
	}
}

func TestDebootstrapAction_VerifyMissingUnstable(t *testing.T) {
	setupDebootstrapScripts(t)

	d := NewDebootstrapAction()
	d.Suite = "v2023"

	err := d.Verify(&debos.Context{CommonContext: &debos.CommonContext{}, RecipeDir: t.TempDir()})
	require.EqualError(t, err, "cannot find debootstrap script for unstable")
}
