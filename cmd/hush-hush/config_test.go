package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg     config
		wantErr string
	}{
		"valid":      {cfg: config{Addr: ":8080", DBPath: "hush-hush.db"}, wantErr: ""},
		"empty addr": {cfg: config{DBPath: "hush-hush.db"}, wantErr: "addr: required"},
		"empty db":   {cfg: config{Addr: ":8080"}, wantErr: "db_path: required"},
		"label":      {cfg: config{Addr: ":8080", DBPath: "x.db", InstanceLabel: "prod / homelab"}, wantErr: ""},
		"label at the cap": {
			cfg:     config{Addr: ":8080", DBPath: "x.db", InstanceLabel: strings.Repeat("a", 40)},
			wantErr: "",
		},
		"label over the cap": {
			cfg:     config{Addr: ":8080", DBPath: "x.db", InstanceLabel: strings.Repeat("a", 41)},
			wantErr: "instance_label: INSTANCE_LABEL must be at most 40 characters",
		},
		"label with a control character": {
			cfg:     config{Addr: ":8080", DBPath: "x.db", InstanceLabel: "prod\nhomelab"},
			wantErr: "instance_label: INSTANCE_LABEL must not contain control characters",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate()

			if tc.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestLoadConfigReadsInstanceLabelFromTheEnvironment(t *testing.T) {
	t.Setenv("INSTANCE_LABEL", "prod / homelab")

	cfg, err := loadConfig()

	require.NoError(t, err)
	assert.Equal(t, "prod / homelab", cfg.InstanceLabel)
}

func TestLoadConfigRejectsAnOverlongInstanceLabelNamingTheSetting(t *testing.T) {
	t.Setenv("INSTANCE_LABEL", strings.Repeat("a", 41))

	_, err := loadConfig()

	require.Error(t, err)
	assert.ErrorContains(t, err, "INSTANCE_LABEL")
}
