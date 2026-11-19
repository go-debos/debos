// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// parseSystemdNspawnVersion extracts the major version from the
// systemd-nspawn --version output, e.g. "systemd 262 (262.1-1)".
func parseSystemdNspawnVersion(output string) (int, error) {
	firstLine, _, _ := strings.Cut(output, "\n")
	fields := strings.Fields(firstLine)
	if len(fields) < 2 || fields[0] != "systemd" {
		return 0, fmt.Errorf("unexpected systemd-nspawn version output %q", firstLine)
	}

	// Strip any suffix such as "~rc1".
	v := fields[1]
	if i := strings.IndexFunc(v, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		v = v[:i]
	}

	version, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("failed to parse systemd-nspawn version %q: %w", fields[1], err)
	}

	return version, nil
}

// systemdNspawnVersion returns the major version of systemd-nspawn. The
// result is cached as it won't change.
var systemdNspawnVersion = sync.OnceValues(func() (int, error) {
	out, err := exec.Command("systemd-nspawn", "--version").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get systemd-nspawn version: %w", err)
	}

	return parseSystemdNspawnVersion(string(out))
})
