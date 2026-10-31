package debos_test

import (
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/require"
)

func TestBasicCommand(t *testing.T) {
	require.NoError(t, debos.Command{}.Run("out", "ls", "-l"))
}
