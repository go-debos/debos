package debos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicCommand(t *testing.T) {
	err := Command{}.Run("out", "ls", "-l")
	require.NoError(t, err)
}
