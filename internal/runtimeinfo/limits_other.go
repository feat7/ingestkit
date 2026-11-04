//go:build !linux && !darwin

package runtimeinfo

import "errors"

func nativeLimits() (uint64, uint64, uint64, error) {
	return 0, 0, 0, errors.New("process limits unavailable on this platform")
}
func nativeFDs() (int, error) {
	return 0, errors.New("descriptor inventory unavailable on this platform")
}
func nativePorts() ([]byte, error) {
	return nil, errors.New("ephemeral-port range unavailable on this platform")
}
