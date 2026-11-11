package debos_test

import (
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/require"
)

func TestBasicCommand(t *testing.T) {
	require.NoError(t, debos.Command{}.Run("out", "ls", "-l"))
}

func TestEmptyCommand(t *testing.T) {
	require.EqualError(t, debos.Command{}.Run("out"), "no command given")
}

func TestUnknownChrootMethod(t *testing.T) {
	cmd := debos.Command{ChrootMethod: 42}
	require.EqualError(t, cmd.Run("out", "ls"), "unknown chroot method 42")
}
