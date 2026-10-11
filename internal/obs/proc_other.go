//go:build !windows && !linux

package obs

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// procStat asks ps (macOS, BSD) for a process's CPU time and resident set size.
func procStat(pid int) (time.Duration, uint64, error) {
	out, err := exec.Command("ps", "-o", "rss=,time=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, 0, err
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return 0, 0, fmt.Errorf("unexpected ps output %q", out)
	}
	kb, _ := strconv.ParseUint(f[0], 10, 64)
	return parsePSTime(f[1]), kb << 10, nil
}

// parsePSTime reads [[dd-]hh:]mm:ss[.cc].
func parsePSTime(s string) time.Duration {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days, _ = strconv.Atoi(d)
		s = rest
	}
	parts := strings.Split(s, ":")
	var total float64
	for _, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		total = total*60 + v
	}
	return time.Duration((float64(days)*86400 + total) * float64(time.Second))
}
