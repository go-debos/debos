// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readProfile closes the profile and returns the events written to disk,
// indexed by span name.
func readProfile(t *testing.T, p *Profile) map[string]traceEvent {
	t.Helper()

	require.NoError(t, p.Close())

	data, err := os.ReadFile(p.filename)
	require.NoError(t, err)

	var trace traceFile
	require.NoError(t, json.Unmarshal(data, &trace))

	events := make(map[string]traceEvent)
	for _, event := range trace.TraceEvents {
		if event.Ph == "X" {
			events[event.Name] = event
		}
	}

	return events
}

// assertContains checks that the outer span covers the whole of the inner one.
func assertContains(t *testing.T, outer traceEvent, inner traceEvent) {
	t.Helper()

	assert.LessOrEqual(t, outer.Ts, inner.Ts,
		"%s should start no later than %s", outer.Name, inner.Name)
	assert.GreaterOrEqual(t, outer.Ts+outer.Dur, inner.Ts+inner.Dur,
		"%s should end no earlier than %s", outer.Name, inner.Name)
}

func TestProfile_nesting(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	require.NoError(t, err)

	p.Phase("Run")
	action := p.Start("an action")
	subaction := p.Start("a sub action")
	subaction.Stop()
	action.Stop()

	events := readProfile(t, p)

	assert.Len(t, events, 4)
	assertContains(t, events["recipe.yaml"], events["Run"])
	assertContains(t, events["Run"], events["an action"])
	assertContains(t, events["an action"], events["a sub action"])

	// the profile is rebased so that it starts at zero
	assert.Equal(t, int64(0), events["recipe.yaml"].Ts)

	assert.Equal(t, profileCatRecipe, events["recipe.yaml"].Cat)
	assert.Equal(t, profileCatPhase, events["Run"].Cat)
	assert.Equal(t, profileCatAction, events["an action"].Cat)
}

func TestProfile_phases(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	require.NoError(t, err)

	p.Phase("Verify")
	p.Start("verified").Stop()

	// repeating the current phase keeps the same span open
	p.Phase("Run")
	p.Phase("Run")
	p.Start("ran").Stop()

	events := readProfile(t, p)

	assert.Len(t, events, 5)
	assertContains(t, events["Verify"], events["verified"])
	assertContains(t, events["Run"], events["ran"])

	// phases do not overlap each other
	assert.LessOrEqual(t, events["Verify"].Ts+events["Verify"].Dur, events["Run"].Ts)
}

// A span left open when the profile is closed is closed along with it.
func TestProfile_unclosedSpan(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	require.NoError(t, err)

	p.Phase("Run")
	p.Start("interrupted")

	events := readProfile(t, p)

	assert.Len(t, events, 3)
	assertContains(t, events["Run"], events["interrupted"])
}

// Every complete event carries a duration, even when the span took no
// measurable time: perfetto refuses to parse one which does not.
func TestProfile_zeroDuration(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	require.NoError(t, err)

	p.Start("an instant action").Stop()
	require.NoError(t, p.Close())

	data, err := os.ReadFile(p.filename)
	require.NoError(t, err)

	/* Decode into maps rather than traceEvent, which cannot tell a
	 * missing duration from a zero one. */
	var trace struct {
		TraceEvents []map[string]json.RawMessage `json:"traceEvents"`
	}
	require.NoError(t, json.Unmarshal(data, &trace))

	for _, event := range trace.TraceEvents {
		if string(event["ph"]) != `"X"` {
			continue
		}
		assert.Contains(t, event, "dur",
			"%s should have a duration", event["name"])
	}
}

func TestProfile_merge(t *testing.T) {
	dir := t.TempDir()

	inner, err := NewProfile(filepath.Join(dir, "inner.json"), "inner.yaml")
	require.NoError(t, err)
	inner.Phase("Run")
	inner.Start("inner action").Stop()
	require.NoError(t, inner.Close())

	outer, err := NewProfile(filepath.Join(dir, "outer.json"), "outer.yaml")
	require.NoError(t, err)
	machine := outer.start("fakemachine", profileCatMachine)
	require.NoError(t, outer.Merge(filepath.Join(dir, "inner.json"), machine))
	machine.Stop()

	events := readProfile(t, outer)

	assert.Len(t, events, 5)
	assertContains(t, events["outer.yaml"], events["fakemachine"])
	assertContains(t, events["fakemachine"], events["inner.yaml"])
	assertContains(t, events["inner.yaml"], events["Run"])
	assertContains(t, events["Run"], events["inner action"])
}

// The merged profile is kept inside its parent span even if the process which
// recorded it disagreed about the time.
func TestProfile_mergeClockSkew(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name  string
		start int64
	}{
		{"skewed into the past", 1},
		{"skewed into the future", 1 << 42},
	}

	for _, test := range tests {
		inner, err := NewProfile(filepath.Join(dir, "inner.json"), "inner.yaml")
		require.NoError(t, err)
		inner.Start("inner action").Stop()
		require.NoError(t, inner.Close())

		// pretend the profile was recorded at a different time
		data, err := os.ReadFile(filepath.Join(dir, "inner.json"))
		require.NoError(t, err)
		var trace traceFile
		require.NoError(t, json.Unmarshal(data, &trace))
		trace.StartTs = test.start
		data, err = json.Marshal(trace)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "inner.json"), data, 0o644))

		outer, err := NewProfile(filepath.Join(dir, "outer.json"), "outer.yaml")
		require.NoError(t, err)
		machine := outer.start("fakemachine", profileCatMachine)
		require.NoError(t, outer.Merge(filepath.Join(dir, "inner.json"), machine))
		machine.Stop()

		events := readProfile(t, outer)

		assertContains(t, events["outer.yaml"], events["fakemachine"])
		assertContains(t, events["fakemachine"], events["inner.yaml"])
		assert.Equal(t, int64(0), events["outer.yaml"].Ts, test.name)
	}
}

func TestProfile_mergeMissingFile(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	require.NoError(t, err)

	machine := p.start("fakemachine", profileCatMachine)
	err = p.Merge(filepath.Join(t.TempDir(), "missing.json"), machine)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProfile_unwritable(t *testing.T) {
	_, err := NewProfile(filepath.Join(t.TempDir(), "missing", "profile.json"), "recipe.yaml")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// Profiling is disabled by leaving the profile nil, so every method has to
// cope with a nil receiver.
func TestProfile_disabled(t *testing.T) {
	var p *Profile

	p.Phase("Run")
	span := p.Start("an action")
	assert.Nil(t, span)
	span.Stop()
	require.NoError(t, p.Merge("missing.json", span))
	require.NoError(t, p.Close())
}
