//go:build linux

package runtimeinfo

import (
	"os"
	"syscall"
)

func nativeLimits() (uint64, uint64, uint64, error) {
	var r syscall.Rlimit
	e := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &r)
	return r.Cur, r.Max, ^uint64(0), e
}
func nativeFDs() (int, error)      { entries, e := os.ReadDir("/proc/self/fd"); return len(entries), e }
func nativePorts() ([]byte, error) { return os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range") }
