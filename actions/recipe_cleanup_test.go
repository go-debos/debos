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
	name                     string
	order                    *[]string
	runErr                   error
	preNoMachineErr          error
	cleanupErr               error
	cleanupCalled            bool
	postMachineCleanupErr    error
	postMachineCleanupCalled bool
}

func (m *mockCleanupAction) record() {
	if m.order != nil {
		*m.order = append(*m.order, m.name)
	}
}

func (m *mockCleanupAction) Run(_ *debos.Context) error {
	return m.runErr
}

func (m *mockCleanupAction) PreNoMachine(_ *debos.Context) error {
	return m.preNoMachineErr
}

func (m *mockCleanupAction) Cleanup(_ *debos.Context) error {
	m.cleanupCalled = true
	m.record()
	return m.cleanupErr
}

func (m *mockCleanupAction) PostMachineCleanup(_ *debos.Context) error {
	m.postMachineCleanupCalled = true
	m.record()
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
	ctx := newTestContext()
	recipe := &RecipeAction{context: *ctx}
	err := recipe.Cleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_Cleanup_calls_run_actions verifies Cleanup is called for every action that ran.
func TestRecipeAction_Cleanup_calls_run_actions(t *testing.T) {
	mock1, mock2 := &mockCleanupAction{}, &mockCleanupAction{}
	ctx := newTestContext()
	recipe := &RecipeAction{
		cleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
		context:        *ctx,
	}
	err := recipe.Cleanup(ctx)
	assert.NoError(t, err)
	assert.True(t, mock1.cleanupCalled)
	assert.True(t, mock2.cleanupCalled)
}

// TestRecipeAction_Cleanup_all_run_on_error verifies all cleanups are attempted and errors returned even when one fails.
func TestRecipeAction_Cleanup_all_run_on_error(t *testing.T) {
	cleanupErr := errors.New("cleanup failed")
	mock1 := &mockCleanupAction{cleanupErr: cleanupErr}
	mock2 := &mockCleanupAction{}
	ctx := newTestContext()
	recipe := &RecipeAction{
		cleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
		context:        *ctx,
	}
	err := recipe.Cleanup(ctx)
	assert.ErrorIs(t, err, cleanupErr)
	assert.True(t, mock1.cleanupCalled)
	assert.True(t, mock2.cleanupCalled)
	// the error is reported by the caller, not the recipe action
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_PostMachineCleanup_noop verifies PostMachineCleanup does nothing when Pre* was never called.
func TestRecipeAction_PostMachineCleanup_noop(t *testing.T) {
	ctx := newTestContext()
	recipe := &RecipeAction{context: *ctx}
	err := recipe.PostMachineCleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_PostMachineCleanup_calls_pre_actions verifies PostMachineCleanup is called for every action whose Pre* ran.
func TestRecipeAction_PostMachineCleanup_calls_pre_actions(t *testing.T) {
	mock1, mock2 := &mockCleanupAction{}, &mockCleanupAction{}
	ctx := newTestContext()
	recipe := &RecipeAction{
		postMachineCleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
		context:                   *ctx,
	}
	err := recipe.PostMachineCleanup(ctx)
	assert.NoError(t, err)
	assert.True(t, mock1.postMachineCleanupCalled)
	assert.True(t, mock2.postMachineCleanupCalled)
}

// TestRecipeAction_PostMachineCleanup_all_run_on_error verifies all PostMachineCleanups run and errors returned even when one fails.
func TestRecipeAction_PostMachineCleanup_all_run_on_error(t *testing.T) {
	cleanupErr := errors.New("post machine cleanup failed")
	mock1 := &mockCleanupAction{postMachineCleanupErr: cleanupErr}
	mock2 := &mockCleanupAction{}
	ctx := newTestContext()
	recipe := &RecipeAction{
		postMachineCleanupActions: []YamlAction{wrapMock(mock1), wrapMock(mock2)},
		context:                   *ctx,
	}
	err := recipe.PostMachineCleanup(ctx)
	assert.ErrorIs(t, err, cleanupErr)
	assert.True(t, mock1.postMachineCleanupCalled)
	assert.True(t, mock2.postMachineCleanupCalled)
	// the error is reported by the caller, not the recipe action
	assert.Equal(t, debos.Success, ctx.State)
}

// TestRecipeAction_Run_cleanup_attempted verifies Cleanup runs in reverse order for every action whose Run was attempted, including the one that failed.
func TestRecipeAction_Run_cleanup_attempted(t *testing.T) {
	var order []string
	runErr := errors.New("run failed")
	mock1 := &mockCleanupAction{name: "a", order: &order}
	mock2 := &mockCleanupAction{name: "b", order: &order, runErr: runErr}
	mock3 := &mockCleanupAction{name: "c", order: &order}
	ctx := newTestContext()
	recipe := &RecipeAction{context: *ctx}
	recipe.Actions.Actions = []YamlAction{wrapMock(mock1), wrapMock(mock2), wrapMock(mock3)}

	err := recipe.Run(ctx)
	assert.ErrorIs(t, err, runErr)

	err = recipe.Cleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, order)
	assert.False(t, mock3.cleanupCalled)
}

// TestRecipeAction_PreNoMachine_cleanup_attempted verifies PostMachineCleanup runs in reverse order for every action whose PreNoMachine was attempted, including the one that failed.
func TestRecipeAction_PreNoMachine_cleanup_attempted(t *testing.T) {
	var order []string
	preErr := errors.New("pre no machine failed")
	mock1 := &mockCleanupAction{name: "a", order: &order}
	mock2 := &mockCleanupAction{name: "b", order: &order, preNoMachineErr: preErr}
	mock3 := &mockCleanupAction{name: "c", order: &order}
	ctx := newTestContext()
	recipe := &RecipeAction{context: *ctx}
	recipe.Actions.Actions = []YamlAction{wrapMock(mock1), wrapMock(mock2), wrapMock(mock3)}

	err := recipe.PreNoMachine(ctx)
	assert.ErrorIs(t, err, preErr)

	err = recipe.PostMachineCleanup(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, order)
	assert.False(t, mock3.postMachineCleanupCalled)
}
