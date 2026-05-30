package debos

import (
	"testing"
)

func TestBasicCommand(t *testing.T) {
	if err := (Command{}).Run("out", "ls", "-l"); err != nil {
		t.Fatalf("command failed: %v", err)
	}
}
