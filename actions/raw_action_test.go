package actions_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
)

func TestRawAction_DefaultOrigin(t *testing.T) {
	// Create a temporary directory for the test
	tmpdir, err := os.MkdirTemp("", "debos-test-raw-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Create a recipe directory
	recipeDir := filepath.Join(tmpdir, "recipe")
	err = os.Mkdir(recipeDir, 0755)
	assert.NoError(t, err)

	// Create a test file in the recipe directory
	testFile := filepath.Join(recipeDir, "bootloader.img")
	testContent := []byte("test bootloader content")
	err = os.WriteFile(testFile, testContent, 0644)
	assert.NoError(t, err)

	// Create a scratch directory
	scratchDir := filepath.Join(tmpdir, "scratch")
	err = os.Mkdir(scratchDir, 0755)
	assert.NoError(t, err)

	// Create a fake image file
	imagePath := filepath.Join(scratchDir, "disk.img")
	// Create an image file with enough space
	imageFile, err := os.Create(imagePath)
	assert.NoError(t, err)
	err = imageFile.Truncate(1024 * 1024) // 1MB
	assert.NoError(t, err)
	imageFile.Close()

	context := &debos.Context{
		CommonContext: &debos.CommonContext{
			Origins:    make(map[string]string),
			Scratchdir: scratchDir,
			Image:      imagePath,
		},
		RecipeDir:    recipeDir,
		Architecture: "amd64",
		SectorSize:   512,
	}

	// Test case 1: Raw action without origin (should default to recipe directory)
	action1 := actions.RawAction{
		Source: "bootloader.img",
		Offset: "0",
	}

	err = action1.Verify(context)
	assert.NoError(t, err, "Verify should pass without origin property")

	err = action1.Run(context)
	assert.NoError(t, err, "Run should pass with default origin (recipe directory)")

	// Verify content was written to the image
	imageContent, err := os.ReadFile(imagePath)
	assert.NoError(t, err)
	assert.Equal(t, testContent, imageContent[:len(testContent)],
		"Written content should match test content")

	// Test case 2: Raw action with explicit 'recipe' origin
	// Reset the image
	imageFile, err = os.Create(imagePath)
	assert.NoError(t, err)
	err = imageFile.Truncate(1024 * 1024)
	assert.NoError(t, err)
	imageFile.Close()

	action2 := actions.RawAction{
		Origin: "recipe",
		Source: "bootloader.img",
		Offset: "0",
	}

	err = action2.Verify(context)
	assert.NoError(t, err, "Verify should pass with explicit 'recipe' origin")

	err = action2.Run(context)
	assert.NoError(t, err, "Run should pass with explicit 'recipe' origin")

	// Verify content was written to the image
	imageContent, err = os.ReadFile(imagePath)
	assert.NoError(t, err)
	assert.Equal(t, testContent, imageContent[:len(testContent)],
		"Written content should match test content with explicit origin")
}

func TestRawAction_EmptySource(t *testing.T) {
	context := &debos.Context{
		CommonContext: &debos.CommonContext{
			Origins: make(map[string]string),
		},
		RecipeDir:    "/tmp/recipe",
		Architecture: "amd64",
	}

	// Test case: Raw action without source property should fail
	action := actions.RawAction{
		Origin: "recipe",
	}

	err := action.Verify(context)
	assert.Error(t, err, "Verify should fail when source is empty")
	assert.Contains(t, err.Error(), "'source' property can't be empty")
}

func TestRawAction_InvalidOrigin(t *testing.T) {
	// Create a temporary directory for the test
	tmpdir, err := os.MkdirTemp("", "debos-test-raw-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	// Create a scratch directory
	scratchDir := filepath.Join(tmpdir, "scratch")
	err = os.Mkdir(scratchDir, 0755)
	assert.NoError(t, err)

	// Create a fake image file
	imagePath := filepath.Join(scratchDir, "disk.img")
	imageFile, err := os.Create(imagePath)
	assert.NoError(t, err)
	err = imageFile.Truncate(1024 * 1024)
	assert.NoError(t, err)
	imageFile.Close()

	context := &debos.Context{
		CommonContext: &debos.CommonContext{
			Origins:    make(map[string]string),
			Scratchdir: scratchDir,
			Image:      imagePath,
		},
		RecipeDir:    tmpdir,
		Architecture: "amd64",
		SectorSize:   512,
	}

	// Test case: Raw action with non-existent origin should fail
	action := actions.RawAction{
		Origin: "non-existent-origin",
		Source: "bootloader.img",
	}

	err = action.Verify(context)
	assert.NoError(t, err, "Verify should pass (origin is checked at runtime)")

	err = action.Run(context)
	assert.Error(t, err, "Run should fail with non-existent origin")
	assert.Contains(t, err.Error(), "origin `non-existent-origin` doesn't exist")
}
