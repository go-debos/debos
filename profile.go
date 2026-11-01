// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

/* Wall-clock profiling of recipe execution.
 *
 * Profiles are written in the Chrome Trace Event format, which is understood
 * by https://speedscope.app (flamegraph) and https://ui.perfetto.dev
 * (timeline) amongst others.
 *
 * Only complete ("X") events are emitted: each carries both a timestamp and a
 * duration, so the order of the events in the file is irrelevant and the tree
 * structure is derived from the nesting of their time ranges. This makes it
 * possible to record spans which are closed out of order (such as the stacked
 * Cleanup methods) and to merge a profile recorded inside a fakemachine into
 * the profile of the host.
 *
 * Timestamps are absolute nanoseconds since the unix epoch whilst recording,
 * which allows profiles from different processes to be merged without any
 * knowledge of when each of them started. They are rebased to start at zero
 * when the profile is written out, and written as fractional microseconds as
 * the format requires. Truncating them to whole microseconds is not enough:
 * an action which takes no measurable time then ends at the same timestamp
 * as the next span starts, and viewers nest it inside that span instead of
 * the one it really belongs to.
 */

const (
	// Category for the span covering the whole recipe
	profileCatRecipe = "recipe"
	// Category for the span covering one action lifecycle phase
	profileCatPhase = "phase"
	// Category for the span covering a single action
	profileCatAction = "action"
	// Category for the span covering the fakemachine run
	profileCatMachine = "fakemachine"
)

// traceEvent is a single event in the Chrome Trace Event format.
type traceEvent struct {
	Name string  `json:"name"`
	Cat  string  `json:"cat,omitempty"`
	Ph   string  `json:"ph"`
	Ts   float64 `json:"ts"`
	/* Always emitted, even when zero: perfetto rejects a complete event
	 * without a duration and counts it as a json_parser_failure without
	 * saying which event it choked on. */
	Dur  float64           `json:"dur"`
	Pid  int               `json:"pid"`
	Tid  int               `json:"tid"`
	Args map[string]string `json:"args,omitempty"`
}

type traceFile struct {
	TraceEvents []traceEvent `json:"traceEvents"`
	/* Absolute time the profile started, in nanoseconds since the unix
	 * epoch. Not part of the trace event format (viewers ignore unknown
	 * keys); it allows a profile to be merged into another one at the
	 * point in time it was really recorded. */
	StartTs int64 `json:"debosProfileStart,omitempty"`
}

// profileEvent is a span which has been closed, with its start and end in
// nanoseconds since the unix epoch.
type profileEvent struct {
	name  string
	cat   string
	start int64
	end   int64
}

// micros converts nanoseconds to the microseconds used by the trace format.
func micros(ns int64) float64 {
	return float64(ns) / float64(time.Microsecond)
}

// nanos converts microseconds from the trace format back to nanoseconds.
func nanos(us float64) int64 {
	return int64(math.Round(us * float64(time.Microsecond)))
}

// Span is a single timed region of the profile. A Span is opened by
// Profile.Start() and closed by Stop(); spans opened whilst another span is
// open are recorded as children of it.
type Span struct {
	profile *Profile
	name    string
	cat     string
	start   time.Time
	/* Earliest time the span may be closed at; raised by Merge() so that a
	 * span always covers everything merged into it. */
	minEnd time.Time
	/* Range covered by the events merged into this span, so that the time
	 * either side of them can be attributed when it is closed. Zero if
	 * nothing has been merged into it. */
	mergedStart time.Time
	mergedEnd   time.Time
}

// Profile records the wall-clock time taken by each action of a recipe.
//
// All methods accept a nil *Profile so that callers do not need to check
// whether profiling is enabled.
type Profile struct {
	filename string
	recipe   string
	events   []profileEvent
	stack    []*Span
	phase    *Span
	root     *Span
}

// NewProfile creates a profile which will be written to filename. The file is
// created immediately so that an unusable path is reported before the recipe
// starts running rather than after it has finished.
func NewProfile(filename string, recipe string) (*Profile, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("couldn't create profile: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("couldn't create profile: %w", err)
	}

	p := &Profile{filename: filename, recipe: recipe}
	p.root = p.start(recipe, profileCatRecipe)

	return p, nil
}

// start opens a span nested inside the innermost currently open span.
func (p *Profile) start(name string, cat string) *Span {
	s := &Span{
		profile: p,
		name:    name,
		cat:     cat,
		start:   time.Now(),
	}
	p.stack = append(p.stack, s)
	return s
}

// Start opens a span for a single action, nested inside the innermost
// currently open span. The returned span must be closed with Stop().
func (p *Profile) Start(name string) *Span {
	if p == nil {
		return nil
	}

	return p.start(name, profileCatAction)
}

// Phase switches the profile to the named action lifecycle phase; spans
// started afterwards are recorded as children of it. Calling Phase with the
// name of the phase which is already open does nothing, so it can be called
// from each iteration of a loop over the actions.
//
// The phase span is returned for the benefit of callers which need to merge
// another profile into it; it does not have to be stopped by the caller.
func (p *Profile) Phase(name string) *Span {
	if p == nil {
		return nil
	}

	if p.phase != nil {
		if p.phase.name == name {
			return p.phase
		}
		p.phase.Stop()
	}

	p.phase = p.start(name, profileCatPhase)

	return p.phase
}

// Stop closes the span. Any spans which were opened after it and not closed
// are closed as well.
func (s *Span) Stop() {
	if s == nil {
		return
	}

	p := s.profile
	now := time.Now()

	/* Close any spans left open inside this one, then this one. */
	for i := len(p.stack) - 1; i >= 0; i-- {
		open := p.stack[i]

		end := now
		if open.minEnd.After(end) {
			end = open.minEnd
		}

		ts := open.start.UnixNano()
		endTs := end.UnixNano()
		p.events = append(p.events, profileEvent{
			name:  open.name,
			cat:   open.cat,
			start: ts,
			end:   endTs,
		})
		p.fillMerged(open, ts, endTs)
		p.stack = p.stack[:i]

		if open == p.phase {
			p.phase = nil
		}

		if open == s {
			break
		}
	}
}

// fillMerged records the time between the start of a span and the first event
// merged into it, and between the last merged event and the end of the span,
// as children of it. For the fakemachine span these are the time the machine
// spends booting before debos starts inside it and the time it takes to shut
// down afterwards, which would otherwise be left as unexplained gaps in a span
// whose children do not add up to its duration.
func (p *Profile) fillMerged(s *Span, ts int64, end int64) {
	if s.mergedStart.IsZero() {
		return
	}

	fillers := []struct {
		name  string
		start int64
		end   int64
	}{
		{s.name + " startup", ts, s.mergedStart.UnixNano()},
		{s.name + " shutdown", s.mergedEnd.UnixNano(), end},
	}

	for _, filler := range fillers {
		if filler.end <= filler.start {
			continue
		}

		p.events = append(p.events, profileEvent{
			name:  filler.name,
			cat:   profileCatMachine,
			start: filler.start,
			end:   filler.end,
		})
	}
}

// Merge reads a profile written by another debos process and records its
// events inside parent, which must still be open. The events keep the time
// they were really recorded at, so the time the other process spent starting
// up and shutting down either side of them is recorded as well.
func (p *Profile) Merge(filename string, parent *Span) error {
	if p == nil || parent == nil {
		return nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("couldn't read profile: %w", err)
	}

	var trace traceFile
	if err := json.Unmarshal(data, &trace); err != nil {
		return fmt.Errorf("couldn't parse profile %s: %w", filename, err)
	}

	/* Timestamps in the file were rebased to start at zero when it was
	 * written; put them back onto the absolute timeline. */
	offset := trace.StartTs

	/* The other process may not agree with us about the time. Shift the
	 * whole profile forwards if it claims to have started before the
	 * parent span did, so that the events stay nested. */
	if skew := parent.start.UnixNano() - offset; skew > 0 {
		offset += skew
	}

	var first, last time.Time
	for _, event := range trace.TraceEvents {
		if event.Ph != "X" {
			continue
		}

		start := offset + nanos(event.Ts)
		end := start + nanos(event.Dur)

		if t := time.Unix(0, start); first.IsZero() || t.Before(first) {
			first = t
		}
		if t := time.Unix(0, end); t.After(last) {
			last = t
		}

		/* The other process ran the same recipe as we did, so its root
		 * span says nothing the span we are merging into does not
		 * already say. Drop it rather than naming the recipe twice in
		 * the tree; it still counts towards the range covered above. */
		if event.Cat == profileCatRecipe {
			continue
		}

		p.events = append(p.events, profileEvent{
			name:  event.Name,
			cat:   event.Cat,
			start: start,
			end:   end,
		})
	}

	/* Make sure the parent span covers everything merged into it, even if
	 * the other process thinks it ran for longer than we measured. */
	if last.After(parent.minEnd) {
		parent.minEnd = last
	}

	/* Remember the range the merged events cover so that the time either
	 * side of them is attributed when the parent span is closed. */
	if !first.IsZero() {
		if parent.mergedStart.IsZero() || first.Before(parent.mergedStart) {
			parent.mergedStart = first
		}
		if last.After(parent.mergedEnd) {
			parent.mergedEnd = last
		}
	}

	return nil
}

// Close closes any spans which are still open, then writes the profile out.
func (p *Profile) Close() error {
	if p == nil {
		return nil
	}

	p.root.Stop()

	/* Rebase the timestamps so that the profile starts at zero. */
	var base int64
	for _, event := range p.events {
		if base == 0 || event.start < base {
			base = event.start
		}
	}

	events := []traceEvent{{
		Name: "process_name",
		Ph:   "M",
		Pid:  1,
		Tid:  1,
		Args: map[string]string{"name": p.recipe},
	}}
	for _, event := range p.events {
		events = append(events, traceEvent{
			Name: event.name,
			Cat:  event.cat,
			Ph:   "X",
			Ts:   micros(event.start - base),
			Dur:  micros(event.end - event.start),
			Pid:  1,
			Tid:  1,
		})
	}

	data, err := json.MarshalIndent(traceFile{TraceEvents: events, StartTs: base}, "", "  ")
	if err != nil {
		return fmt.Errorf("couldn't encode profile: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(p.filename, data, 0644); err != nil {
		return fmt.Errorf("couldn't write profile: %w", err)
	}

	return nil
}
