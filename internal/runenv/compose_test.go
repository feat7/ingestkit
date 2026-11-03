package runenv

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRunenvRejectsUnownedCleanup(t *testing.T) {
	s := &Stack{Project: "someone-else"}
	require.Error(t, s.Destroy(context.Background()))
	require.Error(t, s.Stop(context.Background()))
}
func TestRunenvScopesEveryComposeCommand(t *testing.T) {
	s := &Stack{Project: "ingestkit-test", root: "/repo", token: "owner"}
	args := s.composeArgs("logs")
	require.Contains(t, args, "-p")
	require.Contains(t, args, "ingestkit-test")
	require.Contains(t, args, "/repo/docker-compose.yml")
}
func TestRunenvDiscoversDynamicPorts(t *testing.T) {
	require.Equal(t, "http://127.0.0.1:12345", apiAddress("127.0.0.1:12345\n"))
	require.NotEqual(t, projectName("test"), projectName("test"))
}
