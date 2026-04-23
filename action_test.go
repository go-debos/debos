package debos_test

import (
	"errors"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/assert"
)

func TestHandleError_nilError(t *testing.T) {
	ctx := &debos.Context{CommonContext: &debos.CommonContext{State: debos.Success}}
	result := debos.HandleError(ctx, nil, &debos.BaseAction{}, "Run")
	assert.False(t, result)
	assert.Equal(t, debos.Success, ctx.State)
}

func TestHandleError_withError(t *testing.T) {
	ctx := &debos.Context{CommonContext: &debos.CommonContext{State: debos.Success}}
	result := debos.HandleError(ctx, errors.New("test error"), &debos.BaseAction{}, "Run")
	assert.True(t, result)
	assert.Equal(t, debos.Failed, ctx.State)
}
