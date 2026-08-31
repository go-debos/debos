package debos_test

import (
	"testing"

	"github.com/go-debos/debos"
)

func TestBasicCommand(_ *testing.T) {
	_ = debos.Command{}.Run("out", "ls", "-l")
}
