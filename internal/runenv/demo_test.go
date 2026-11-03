package runenv

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCheckoutDemoReconciles(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	require.NoError(t, reconcileIDs([]uuid.UUID{a, b}, []uuid.UUID{b, a}))
	require.Error(t, reconcileIDs([]uuid.UUID{a, b}, []uuid.UUID{a}))
	require.Error(t, reconcileIDs([]uuid.UUID{a}, []uuid.UUID{a, b}))
	require.Error(t, reconcileIDs([]uuid.UUID{a}, []uuid.UUID{a, a}))
}
