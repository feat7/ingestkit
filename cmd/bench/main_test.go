package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMeasuredModesUseDurableOwnedProfile(t *testing.T) {
	for _, mode := range []string{"pipeline", "database"} {
		o := measuredOptions(mode)
		require.Equal(t, "selfhost", o.Profile)
		require.True(t, o.ExposeDependencies)
	}
}
func TestCLIRejectsInvalidFlagsBeforeLaunching(t *testing.T) {
	for _, args := range [][]string{{"-unknown"}, {"-mode", "study", "-suite", "wrong"}, {"-suite", "connections"}, {"-mode", "pipeline", "-rate", "50000", "-duration", "60s"}, {"-mode", "pipeline", "-payload", "16384", "-batch", "100"}} {
		require.Error(t, runArgs(args))
	}
}
