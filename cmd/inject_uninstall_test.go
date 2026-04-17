// cmd/inject_uninstall_test.go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectUninstall_FlagMutualExclusion(t *testing.T) {
	tests := []struct {
		name      string
		setupVars func()
		wantErr   string
	}{
		{
			name: "uninstall with sync",
			setupVars: func() {
				injectUninstall = true
				injectSync = true
				injectNoSync = false
			},
			wantErr: "--uninstall is incompatible",
		},
		{
			name: "uninstall with no-sync",
			setupVars: func() {
				injectUninstall = true
				injectSync = false
				injectNoSync = true
			},
			wantErr: "--uninstall is incompatible",
		},
		{
			name: "sync with no-sync (existing check)",
			setupVars: func() {
				injectUninstall = false
				injectSync = true
				injectNoSync = true
			},
			wantErr: "--sync and --no-sync are mutually exclusive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupVars()
			t.Cleanup(func() {
				injectUninstall = false
				injectSync = false
				injectNoSync = false
			})
			err := injectCmd.RunE(injectCmd, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestInjectUninstall_FlagsExist(t *testing.T) {
	require.NotNil(t, injectCmd.Flags().Lookup("uninstall"), "--uninstall flag must be registered")
	require.NotNil(t, injectCmd.Flags().Lookup("stats"), "--stats flag must be registered")
}

func TestInjectUninstall_StatsNotMutuallyExclusive(t *testing.T) {
	// --uninstall + --stats must not be rejected by the mutual exclusion check.
	// The command will proceed past validation and may fail later (e.g. XDG paths),
	// but must never return the "--uninstall is incompatible" error.
	injectUninstall = true
	injectStats = true
	injectSync = false
	injectNoSync = false
	t.Cleanup(func() {
		injectUninstall = false
		injectStats = false
	})
	err := injectCmd.RunE(injectCmd, nil)
	// The command may error (XDG/adapter load) but NOT with the mutual-exclusion message.
	if err != nil {
		assert.NotContains(t, err.Error(), "--uninstall is incompatible")
		assert.NotContains(t, err.Error(), "mutually exclusive")
	}
}
