package debos

import (
	"testing"
)

func TestBasicCommand(t *testing.T) {
	Command{}.Run("out", "ls", "-l")
}
