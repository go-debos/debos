// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// readProfile closes the profile and returns the events written to disk,
// indexed by span name.
func readProfile(t *testing.T, p *Profile) map[string]traceEvent {
	assert.NoError(t, p.Close())

	data, err := os.ReadFile(p.filename)
	assert.NoError(t, err)

	var trace traceFile
	assert.NoError(t, json.Unmarshal(data, &trace))

	events := make(map[string]traceEvent)
	for _, event := range trace.TraceEvents {
		if event.Ph == "X" {
			events[event.Name] = event
		}
	}

	return events
}

// end returns the time the span ended at, in nanoseconds since the start of
// the profile. Rounding it to whole nanoseconds hides the error introduced by
// adding up the fractional microseconds in the file.
func end(event traceEvent) int64 {
	return nanos(event.Ts + event.Dur)
}

// assertContains checks that the outer span covers the whole of the inner one.
func assertContains(t *testing.T, outer traceEvent, inner traceEvent) {
	assert.LessOrEqual(t, outer.Ts, inner.Ts,
		"%s should start no later than %s", outer.Name, inner.Name)
	assert.GreaterOrEqual(t, end(outer), end(inner),
		"%s should end no earlier than %s", outer.Name, inner.Name)
}

func TestProfile_nesting(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

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
	assert.Equal(t, 0.0, events["recipe.yaml"].Ts)

	assert.Equal(t, profileCatRecipe, events["recipe.yaml"].Cat)
	assert.Equal(t, profileCatPhase, events["Run"].Cat)
	assert.Equal(t, profileCatAction, events["an action"].Cat)
}

func TestProfile_phases(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

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
	assert.LessOrEqual(t, end(events["Verify"]), nanos(events["Run"].Ts))
}

// A span left open when the profile is closed is closed along with it.
func TestProfile_unclosedSpan(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

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
	assert.NoError(t, err)

	p.Start("an instant action").Stop()
	assert.NoError(t, p.Close())

	data, err := os.ReadFile(p.filename)
	assert.NoError(t, err)

	/* Decode into maps rather than traceEvent, which cannot tell a
	 * missing duration from a zero one. */
	var trace struct {
		TraceEvents []map[string]json.RawMessage `json:"traceEvents"`
	}
	assert.NoError(t, json.Unmarshal(data, &trace))

	for _, event := range trace.TraceEvents {
		if string(event["ph"]) != `"X"` {
			continue
		}
		assert.Contains(t, event, "dur",
			"%s should have a duration", event["name"])
	}
}

// An action which takes no measurable time at the end of a phase stays inside
// that phase rather than appearing to start at the same time as the next one.
func TestProfile_zeroDurationAtPhaseEnd(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

	p.Phase("PreMachine")
	p.Start("an instant action").Stop()
	p.Phase("fakemachine")

	events := readProfile(t, p)

	assertContains(t, events["PreMachine"], events["an instant action"])
	assert.Less(t, end(events["an instant action"]), nanos(events["fakemachine"].Ts))
}

func TestProfile_merge(t *testing.T) {
	dir := t.TempDir()

	inner, err := NewProfile(filepath.Join(dir, "inner.json"), "inner.yaml")
	assert.NoError(t, err)
	inner.Phase("Run")
	inner.Start("inner action").Stop()
	assert.NoError(t, inner.Close())

	outer, err := NewProfile(filepath.Join(dir, "outer.json"), "outer.yaml")
	assert.NoError(t, err)
	machine := outer.start("fakemachine", profileCatMachine)
	assert.NoError(t, outer.Merge(filepath.Join(dir, "inner.json"), machine))
	machine.Stop()

	events := readProfile(t, outer)

	assert.Len(t, events, 5)
	assertContains(t, events["outer.yaml"], events["fakemachine"])
	assertContains(t, events["fakemachine"], events["Run"])
	assertContains(t, events["Run"], events["inner action"])

	/* the time between the last merged event and the end of the span is
	 * attributed to the other process shutting down */
	assertContains(t, events["fakemachine"], events["fakemachine shutdown"])

	/* the root span of the merged profile covers the same recipe as the
	 * span it was merged into, so it is not recorded again */
	assert.NotContains(t, events, "inner.yaml")
}

// The time before and after the events of a merged profile is attributed to
// the other process starting up and shutting down.
func TestProfile_mergeStartupShutdown(t *testing.T) {
	dir := t.TempDir()

	inner, err := NewProfile(filepath.Join(dir, "inner.json"), "inner.yaml")
	assert.NoError(t, err)
	inner.Phase("Run")
	inner.Start("inner action").Stop()
	assert.NoError(t, inner.Close())

	outer, err := NewProfile(filepath.Join(dir, "outer.json"), "outer.yaml")
	assert.NoError(t, err)
	machine := outer.start("fakemachine", profileCatMachine)
	/* pretend the machine took a while to boot and to shut down */
	machine.start = machine.start.Add(-time.Second)
	assert.NoError(t, outer.Merge(filepath.Join(dir, "inner.json"), machine))
	machine.minEnd = machine.minEnd.Add(time.Second)
	machine.Stop()

	events := readProfile(t, outer)

	startup := events["fakemachine startup"]
	shutdown := events["fakemachine shutdown"]

	assertContains(t, events["fakemachine"], startup)
	assertContains(t, events["fakemachine"], shutdown)
	assert.Equal(t, events["fakemachine"].Ts, startup.Ts)
	assert.Equal(t, end(events["fakemachine"]), end(shutdown))

	// they cover the time outside the merged events, not the events
	assert.LessOrEqual(t, end(startup), nanos(events["Run"].Ts))
	assert.GreaterOrEqual(t, nanos(shutdown.Ts), end(events["Run"]))

	/* not exactly a second, as the profiles were not written at exactly
	 * the times the spans were opened and closed at */
	assert.Greater(t, startup.Dur, micros(int64(time.Second/2)))
	assert.Greater(t, shutdown.Dur, micros(int64(time.Second/2)))
}

// Nothing is merged into an ordinary span, so it gets no startup or shutdown
// entries.
func TestProfile_noStartupShutdownWithoutMerge(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

	p.Phase("Run")
	p.Start("an action").Stop()

	events := readProfile(t, p)

	assert.Len(t, events, 3)
}

// The merged profile is kept inside its parent span even if the process which
// recorded it disagreed about the time.
func TestProfile_mergeClockSkew(t *testing.T) {
	dir := t.TempDir()

	var tests = []struct {
		name  string
		start int64
	}{
		{"skewed into the past", 1},
		{"skewed into the future", (1 << 42) * int64(time.Microsecond)},
	}

	for _, test := range tests {
		inner, err := NewProfile(filepath.Join(dir, "inner.json"), "inner.yaml")
		assert.NoError(t, err)
		inner.Start("inner action").Stop()
		assert.NoError(t, inner.Close())

		// pretend the profile was recorded at a different time
		data, err := os.ReadFile(filepath.Join(dir, "inner.json"))
		assert.NoError(t, err)
		var trace traceFile
		assert.NoError(t, json.Unmarshal(data, &trace))
		trace.StartTs = test.start
		data, err = json.Marshal(trace)
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "inner.json"), data, 0644))

		outer, err := NewProfile(filepath.Join(dir, "outer.json"), "outer.yaml")
		assert.NoError(t, err)
		machine := outer.start("fakemachine", profileCatMachine)
		assert.NoError(t, outer.Merge(filepath.Join(dir, "inner.json"), machine))
		machine.Stop()

		events := readProfile(t, outer)

		assertContains(t, events["outer.yaml"], events["fakemachine"])
		assertContains(t, events["fakemachine"], events["inner action"])
		assert.Equal(t, 0.0, events["outer.yaml"].Ts, test.name)
	}
}

func TestProfile_mergeMissingFile(t *testing.T) {
	p, err := NewProfile(filepath.Join(t.TempDir(), "profile.json"), "recipe.yaml")
	assert.NoError(t, err)

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
	assert.NoError(t, p.Merge("missing.json", span))
	assert.NoError(t, p.Close())
}
