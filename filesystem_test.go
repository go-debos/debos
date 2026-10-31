package debos_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/require"
)

func TestCopyTreeSymlink(t *testing.T) {
	for _, existing := range []string{"missing", "symlink", "symlink to directory", "file", "directory"} {
		t.Run(existing, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			require.NoError(t, os.Symlink("new-target", filepath.Join(source, "link")))
			target := filepath.Join(destination, "link")
			var oldDirectory string
			switch existing {
			case "symlink":
				require.NoError(t, os.Symlink("old-target", target))
			case "symlink to directory":
				oldDirectory = t.TempDir()
				require.NoError(t, os.Symlink(oldDirectory, target))
			case "file":
				require.NoError(t, os.WriteFile(target, []byte("old contents"), 0644))
			case "directory":
				require.NoError(t, os.Mkdir(target, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(target, "keep"), []byte("keep contents"), 0644))
			}
			err := debos.CopyTree(source, destination)
			if existing == "directory" {
				require.NoError(t, err)
				info, statErr := os.Stat(target)
				require.NoError(t, statErr)
				require.True(t, info.IsDir())
				contents, readErr := os.ReadFile(filepath.Join(target, "keep"))
				require.NoError(t, readErr)
				require.Equal(t, "keep contents", string(contents))
			} else {
				require.NoError(t, err)
				link, readErr := os.Readlink(target)
				require.NoError(t, readErr)
				require.Equal(t, "new-target", link)
			}
			if oldDirectory != "" {
				info, statErr := os.Stat(oldDirectory)
				require.NoError(t, statErr)
				require.True(t, info.IsDir())
			}
			entries, readErr := os.ReadDir(destination)
			require.NoError(t, readErr)
			require.Len(t, entries, 1)
		})
	}
}
