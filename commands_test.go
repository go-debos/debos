package debos

import (
	"testing"
)

func TestBasicCommand(_ *testing.T) {
	_ = Command{}.Run("out", "ls", "-l")
}
