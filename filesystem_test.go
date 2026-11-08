package debos_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/require"
)

// copyTreeWithLink copies a tree holding one symlink named "link" that points
// to "new-target" and returns the destination tree.
func copyTreeWithLink(t *testing.T, prepare func(target string)) string {
	t.Helper()

	source, destination := t.TempDir(), t.TempDir()
	require.NoError(t, os.Symlink("new-target", filepath.Join(source, "link")))
	if prepare != nil {
		prepare(filepath.Join(destination, "link"))
	}
	require.NoError(t, debos.CopyTree(source, destination))

	return destination
}

func requireOnlyLink(t *testing.T, destination string) {
	t.Helper()

	link, err := os.Readlink(filepath.Join(destination, "link"))
	require.NoError(t, err)
	require.Equal(t, "new-target", link)

	entries, err := os.ReadDir(destination)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestCopyTreeSymlinkMissing(t *testing.T) {
	requireOnlyLink(t, copyTreeWithLink(t, nil))
}

func TestCopyTreeSymlinkReplacesSymlink(t *testing.T) {
	destination := copyTreeWithLink(t, func(target string) {
		require.NoError(t, os.Symlink("old-target", target))
	})
	requireOnlyLink(t, destination)
}

func TestCopyTreeSymlinkReplacesSymlinkToDirectory(t *testing.T) {
	oldDirectory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(oldDirectory, "keep"), []byte("keep contents"), 0o600))

	destination := copyTreeWithLink(t, func(target string) {
		require.NoError(t, os.Symlink(oldDirectory, target))
	})
	requireOnlyLink(t, destination)

	// The directory the old link pointed to is left alone.
	contents, err := os.ReadFile(filepath.Join(oldDirectory, "keep"))
	require.NoError(t, err)
	require.Equal(t, "keep contents", string(contents))
}

func TestCopyTreeSymlinkReplacesFile(t *testing.T) {
	destination := copyTreeWithLink(t, func(target string) {
		require.NoError(t, os.WriteFile(target, []byte("old contents"), 0o600))
	})
	requireOnlyLink(t, destination)
}

func TestCopyTreeSymlinkReplacesDirectory(t *testing.T) {
	destination := copyTreeWithLink(t, func(target string) {
		require.NoError(t, os.Mkdir(target, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(target, "old"), []byte("old contents"), 0o600))
	})
	requireOnlyLink(t, destination)
}

func TestCopyTreeSymlinkedSourceKeepsDestination(t *testing.T) {
	sourceDirectory, destination := t.TempDir(), t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.Symlink(sourceDirectory, source))
	require.NoError(t, os.WriteFile(filepath.Join(destination, "keep"), []byte("keep contents"), 0o600))

	require.NoError(t, debos.CopyTree(source, destination))

	contents, err := os.ReadFile(filepath.Join(destination, "keep"))
	require.NoError(t, err)
	require.Equal(t, "keep contents", string(contents))
}

func TestCopyTreeSymlinkedSourceCreatesMissingDestination(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.Symlink(t.TempDir(), source))
	destination := filepath.Join(t.TempDir(), "destination")

	require.NoError(t, debos.CopyTree(source, destination))

	info, err := os.Lstat(destination)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
}
