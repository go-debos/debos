// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package debos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSystemdNspawnVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		version int
		wantErr bool
	}{
		{
			name:    "debian",
			output:  "systemd 262 (262.1-1)\n+PAM +AUDIT +SELINUX\n",
			version: 262,
		},
		{
			name:    "release candidate",
			output:  "systemd 261~rc2 (261~rc2-1)\n",
			version: 261,
		},
		{
			name:    "no newline",
			output:  "systemd 252 (252.39-1~deb12u1)",
			version: 252,
		},
		{
			name:    "empty",
			output:  "",
			wantErr: true,
		},
		{
			name:    "missing version",
			output:  "systemd\n",
			wantErr: true,
		},
		{
			name:    "unexpected program",
			output:  "foo 262\n",
			wantErr: true,
		},
		{
			name:    "non-numeric version",
			output:  "systemd abc (abc)\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseSystemdNspawnVersion(tt.output)
			if tt.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.version, version)
		})
	}
}
