package runtimeinfo

import (
	"strconv"
	"strings"
)

type Limit struct {
	Status string  `json:"status"`
	Value  *uint64 `json:"value"`
}
type ProcessLimits struct {
	SoftFDLimit    Limit `json:"soft_fd_limit"`
	HardFDLimit    Limit `json:"hard_fd_limit"`
	OpenFDs        Limit `json:"open_fds"`
	EphemeralFirst Limit `json:"ephemeral_first"`
	EphemeralLast  Limit `json:"ephemeral_last"`
}

func CaptureProcessLimits() ProcessLimits { return captureLimits(nativeLimits, nativeFDs, nativePorts) }
func captureLimits(get func() (uint64, uint64, uint64, error), fds func() (int, error), ports func() ([]byte, error)) ProcessLimits {
	unknown := Limit{Status: "unavailable"}
	r := ProcessLimits{SoftFDLimit: unknown, HardFDLimit: unknown, OpenFDs: unknown, EphemeralFirst: unknown, EphemeralLast: unknown}
	known := func(value uint64) Limit { return Limit{Status: "known", Value: &value} }
	soft, hard, infinity, e := get()
	if e == nil {
		if soft == infinity {
			r.SoftFDLimit = Limit{Status: "unlimited"}
		} else {
			r.SoftFDLimit = known(soft)
		}
		if hard == infinity {
			r.HardFDLimit = Limit{Status: "unlimited"}
		} else {
			r.HardFDLimit = known(hard)
		}
	}
	if count, e := fds(); e == nil && count >= 0 {
		r.OpenFDs = known(uint64(count))
	}
	if raw, e := ports(); e == nil {
		parts := strings.Fields(string(raw))
		if len(parts) == 2 {
			a, ae := strconv.ParseUint(parts[0], 10, 16)
			b, be := strconv.ParseUint(parts[1], 10, 16)
			if ae == nil && be == nil && a > 0 && a <= b {
				r.EphemeralFirst = known(a)
				r.EphemeralLast = known(b)
			}
		}
	}
	return r
}
