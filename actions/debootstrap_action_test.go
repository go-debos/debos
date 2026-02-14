package actions_test

import (
	"runtime"
	"testing"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
)

func TestDebootstrapAction_Components_Multiple(t *testing.T) {
	context := &debos.Context{
		CommonContext: &debos.CommonContext{},
		Architecture:  runtime.GOARCH,
	}

	action := actions.NewDebootstrapAction()
	action.Suite = "trixie"
	action.Mirror = "https://deb.debian.org/debian"
	action.Components = []string{"main", "contrib", "non-free"}

	cmdline := action.BuildDebootstrapCommand(context)

	assert.Contains(t, cmdline, "--components=main,contrib,non-free")
}

func TestDebootstrapAction_Components_Default(t *testing.T) {
	context := &debos.Context{
		CommonContext: &debos.CommonContext{},
		Architecture:  runtime.GOARCH,
	}

	action := actions.NewDebootstrapAction()
	action.Suite = "trixie"

	cmdline := action.BuildDebootstrapCommand(context)

	assert.Contains(t, cmdline, "--components=main")
}

func TestDebootstrapAction_Mirror_Custom(t *testing.T) {
	context := &debos.Context{
		CommonContext: &debos.CommonContext{},
		Architecture:  runtime.GOARCH,
	}

	action := actions.NewDebootstrapAction()
	action.Suite = "trixie"
	action.Mirror = "https://example.com/debian"

	cmdline := action.BuildDebootstrapCommand(context)

	assert.Contains(t, cmdline, "https://example.com/debian")
}

func TestDebootstrapAction_Mirror_Default(t *testing.T) {
	context := &debos.Context{
		CommonContext: &debos.CommonContext{},
		Architecture:  runtime.GOARCH,
	}

	action := actions.NewDebootstrapAction()
	action.Suite = "trixie"

	cmdline := action.BuildDebootstrapCommand(context)

	assert.Contains(t, cmdline, "https://deb.debian.org/debian")
}
