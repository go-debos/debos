package actions

import (
	"errors"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/assert"
)

// mockCleanupAction is a test-only action that tracks calls and returns configurable errors.
type mockCleanupAction struct {
	debos.BaseAction
	cleanupErr               error
	cleanupCalled            bool
	postMachineCleanupErr    error
	postMachineCleanupCalled bool
}

func (m *mockCleanupAction) Cleanup(_ *debos.Context) error {
	m.cleanupCalled = true
	return m.cleanupErr
}

func (m *mockCleanupAction) PostMachineCleanup(_ *debos.Context) error {
	m.postMachineCleanupCalled = true
	return m.postMachineCleanupErr
}

func wrapMock(a debos.Action) YamlAction {
	return YamlAction{Action: a}
}

func newTestContext() *debos.Context {
	return &debos.Context{
		CommonContext: &debos.CommonContext{State: debos.Success},
	}
}

// TestRecipeAction_Cleanup_noop verifies Cleanup does nothing when Run was never attempted.
func TestRecipeAction_Cleanup_noop(t *testing.T) {
	recipe := &RecipeAction{}
	ctx := newTestContext()
	err := recipe.Cleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_Cleanup_calls_run_actions verifies Cleanup is called for every action that ran.
func TestRecipeAction_Cleanup_calls_run_actions(t *testing.T) {
	mock1, mock2 := &mockCleanupAction{}, &mockCleanupAction{}
	recipe := &RecipeAction{
		cleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
	}
	ctx := newTestContext()
	err := recipe.Cleanup(ctx)
	assert.NoError(t, err)
	assert.True(t, mock1.cleanupCalled)
	assert.True(t, mock2.cleanupCalled)
}

// TestRecipeAction_Cleanup_all_run_on_error verifies all cleanups are attempted even when one fails.
func TestRecipeAction_Cleanup_all_run_on_error(t *testing.T) {
	mock1 := &mockCleanupAction{cleanupErr: errors.New("cleanup failed")}
	mock2 := &mockCleanupAction{}
	recipe := &RecipeAction{
		cleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
	}
	ctx := newTestContext()
	err := recipe.Cleanup(ctx)
	assert.Error(t, err)
	assert.True(t, mock1.cleanupCalled)
	assert.True(t, mock2.cleanupCalled)
	assert.Equal(t, debos.Failed, ctx.State)
}

// TestRecipeAction_PostMachineCleanup_noop verifies PostMachineCleanup does nothing when Pre* was never called.
func TestRecipeAction_PostMachineCleanup_noop(t *testing.T) {
	recipe := &RecipeAction{}
	ctx := newTestContext()
	err := recipe.PostMachineCleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_PostMachineCleanup_calls_pre_actions verifies PostMachineCleanup is called for every action whose Pre* ran.
func TestRecipeAction_PostMachineCleanup_calls_pre_actions(t *testing.T) {
	mock1, mock2 := &mockCleanupAction{}, &mockCleanupAction{}
	recipe := &RecipeAction{
		postMachineCleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
	}
	ctx := newTestContext()
	err := recipe.PostMachineCleanup(ctx)
	assert.NoError(t, err)
	assert.True(t, mock1.postMachineCleanupCalled)
	assert.True(t, mock2.postMachineCleanupCalled)
}

// TestRecipeAction_PostMachineCleanup_all_run_on_error verifies all PostMachineCleanups run even when one fails.
func TestRecipeAction_PostMachineCleanup_all_run_on_error(t *testing.T) {
	mock1 := &mockCleanupAction{postMachineCleanupErr: errors.New("post machine cleanup failed")}
	mock2 := &mockCleanupAction{}
	recipe := &RecipeAction{
		postMachineCleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
	}
	ctx := newTestContext()
	err := recipe.PostMachineCleanup(ctx)
	assert.Error(t, err)
	assert.True(t, mock1.postMachineCleanupCalled)
	assert.True(t, mock2.postMachineCleanupCalled)
	assert.Equal(t, debos.Failed, ctx.State)
}
