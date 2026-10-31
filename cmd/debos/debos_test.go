// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHostTimezone(t *testing.T) {
	// TZ takes precedence over the configuration of the host
	t.Setenv("TZ", "America/New_York")
	assert.Equal(t, "America/New_York", hostTimezoneName())

	newYork, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	_, offset := time.Now().In(newYork).Zone()
	assert.Equal(t, fmt.Sprintf("America/New_York:%d", offset), hostTimezone())

	t.Setenv("TZ", ":Europe/London")
	assert.Equal(t, "Europe/London", hostTimezoneName())

	// The timezone of the host may not be nameable, but must always have an
	// offset which can be parsed back by setLogTimezone()
	t.Setenv("TZ", "")
	_, hostOffset, found := strings.Cut(hostTimezone(), ":")
	assert.True(t, found)
	_, err = strconv.Atoi(hostOffset)
	assert.NoError(t, err)
}

func TestSetLogTimezone(t *testing.T) {
	local := time.Local
	defer func() { time.Local = local }()

	for _, test := range []struct {
		timezone string
		offset   int
	}{
		// Resolved by name
		{"Europe/London:0", 3600},
		{"America/New_York:0", -14400},
		// Unresolvable name, falls back to the given offset
		{"GMT0BST,M3.5.0/1,M10.5.0:7200", 7200},
		{":-3600", -3600},
	} {
		time.Local = time.UTC
		t.Setenv(hostTimezoneVariable, test.timezone)

		setLogTimezone()

		_, offset := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC).Local().Zone()
		assert.Equal(t, test.offset, offset, "timezone %q", test.timezone)
	}
}

func TestSetLogTimezoneUnset(t *testing.T) {
	local := time.Local
	defer func() { time.Local = local }()

	// Nothing to do when running on the host
	time.Local = time.UTC
	t.Setenv(hostTimezoneVariable, "")

	setLogTimezone()

	assert.Equal(t, time.UTC, time.Local)
}
