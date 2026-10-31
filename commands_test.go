package debos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicCommand(t *testing.T) {
	require.NoError(t, Command{}.Run("out", "ls", "-l"))
}
