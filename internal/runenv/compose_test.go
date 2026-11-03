package runenv

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRunenvRejectsChangedProjectIdentity(t *testing.T) {
	s := &Stack{Project: "ingestkit-other", identity: "ingestkit-original", token: "owner", owned: true}
	_, err := s.Compose(context.Background(), "logs")
	require.ErrorContains(t, err, "identity")
}

func TestFailedStartupCleansUpAndReportsEvidence(t *testing.T) {
	for _, cancelDuringPort := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelDuringPort), func(t *testing.T) {
			dir := t.TempDir()
			trace := filepath.Join(dir, "trace")
			fake := filepath.Join(dir, "docker")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_TRACE"
case "$*" in
 *" up "*) exit 0;;
 *" port "*) if [ "$FAKE_CANCEL" = "1" ]; then exec sleep 5; fi; echo discovery-failed >&2; exit 1;;
 *" down "*) echo cleanup-failed >&2; exit 1;;
esac
exit 0
`
			require.NoError(t, os.WriteFile(fake, []byte(script), 0700))
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("FAKE_TRACE", trace)
			t.Setenv("FAKE_CANCEL", "0")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if cancelDuringPort {
				t.Setenv("FAKE_CANCEL", "1")
				go func() {
					for ctx.Err() == nil {
						raw, _ := os.ReadFile(trace)
						if strings.Contains(string(raw), " port ") {
							cancel()
							return
						}
						time.Sleep(time.Millisecond)
					}
				}()
			}
			s, e := Start(ctx, Options{Purpose: "cleanup-probe", OutputDirectory: dir})
			require.Nil(t, s)
			require.Error(t, e)
			require.ErrorContains(t, e, dir)
			require.ErrorContains(t, e, "cleanup-failed")
			raw, _ := os.ReadFile(trace)
			require.Contains(t, string(raw), " down -v")
			require.FileExists(t, filepath.Join(dir, "services.log"))
		})
	}
}
