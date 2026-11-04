//go:build darwin

package runtimeinfo

import (
	"context"
	"math"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func nativeLimits() (uint64, uint64, uint64, error) {
	var r syscall.Rlimit
	e := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &r)
	return r.Cur, r.Max, math.MaxInt64, e
}
func nativeFDs() (int, error) { entries, e := os.ReadDir("/dev/fd"); return len(entries), e }
func nativePorts() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "sysctl", "-n", "net.inet.ip.portrange.first", "net.inet.ip.portrange.last").Output()
}
