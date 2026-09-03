package main

import (
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
