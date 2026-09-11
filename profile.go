// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"encoding/json"
	"fmt"
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
 * Timestamps are absolute microseconds since the unix epoch whilst recording,
 * which allows profiles from different processes to be merged without any
 * knowledge of when each of them started. They are rebased to start at zero
 * when the profile is written out.
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
	Name string `json:"name"`
	Cat  string `json:"cat,omitempty"`
	Ph   string `json:"ph"`
	Ts   int64  `json:"ts"`
	/* Always emitted, even when zero: perfetto rejects a complete event
	 * without a duration and counts it as a json_parser_failure without
	 * saying which event it choked on. */
	Dur  int64             `json:"dur"`
	Pid  int               `json:"pid"`
	Tid  int               `json:"tid"`
	Args map[string]string `json:"args,omitempty"`
}

type traceFile struct {
	TraceEvents []traceEvent `json:"traceEvents"`
	/* Absolute time the profile started, in microseconds since the unix
	 * epoch. Not part of the trace event format (viewers ignore unknown
	 * keys); it allows a profile to be merged into another one at the
	 * point in time it was really recorded. */
	StartTs int64 `json:"debosProfileStart,omitempty"`
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
}

// Profile records the wall-clock time taken by each action of a recipe.
//
// All methods accept a nil *Profile so that callers do not need to check
// whether profiling is enabled.
type Profile struct {
	filename string
	events   []traceEvent
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

	p := &Profile{filename: filename}

	p.events = append(p.events, traceEvent{
		Name: "process_name",
		Ph:   "M",
		Pid:  1,
		Tid:  1,
		Args: map[string]string{"name": recipe},
	})

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

		/* Derive the duration from the truncated timestamps rather
		 * than truncating it separately, otherwise a span can be
		 * recorded as ending a microsecond after its parent. */
		ts := open.start.UnixMicro()
		p.events = append(p.events, traceEvent{
			Name: open.name,
			Cat:  open.cat,
			Ph:   "X",
			Ts:   ts,
			Dur:  end.UnixMicro() - ts,
			Pid:  1,
			Tid:  1,
		})
		p.stack = p.stack[:i]

		if open == p.phase {
			p.phase = nil
		}

		if open == s {
			break
		}
	}
}

// Merge reads a profile written by another debos process and records its
// events inside parent, which must still be open. The events keep the time
// they were really recorded at, so for example the time a fakemachine spends
// booting before debos starts inside it is visible as a gap.
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
	if skew := parent.start.UnixMicro() - offset; skew > 0 {
		offset += skew
	}

	end := parent.minEnd
	for _, event := range trace.TraceEvents {
		if event.Ph != "X" {
			continue
		}

		event.Ts += offset
		event.Pid = 1
		event.Tid = 1

		if eventEnd := time.UnixMicro(event.Ts + event.Dur); eventEnd.After(end) {
			end = eventEnd
		}

		/* The other process ran the same recipe as we did, so its root
		 * span says nothing the span we are merging into does not
		 * already say. Drop it rather than naming the recipe twice in
		 * the tree; it still counts towards the range covered above. */
		if event.Cat == profileCatRecipe {
			continue
		}

		p.events = append(p.events, event)
	}

	/* Make sure the parent span covers everything merged into it, even if
	 * the other process thinks it ran for longer than we measured. */
	parent.minEnd = end

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
		if event.Ph == "X" && (base == 0 || event.Ts < base) {
			base = event.Ts
		}
	}
	for i := range p.events {
		if p.events[i].Ph == "X" {
			p.events[i].Ts -= base
		}
	}

	data, err := json.Marshal(traceFile{TraceEvents: p.events, StartTs: base})
	if err != nil {
		return fmt.Errorf("couldn't encode profile: %w", err)
	}

	if err := os.WriteFile(p.filename, data, 0644); err != nil {
		return fmt.Errorf("couldn't write profile: %w", err)
	}

	return nil
}
