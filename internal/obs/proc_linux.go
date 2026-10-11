package obs

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const clockTicks = 100 // USER_HZ on every mainstream Linux

// procStat returns a process's total CPU time and resident set size.
func procStat(pid int) (time.Duration, uint64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, err
	}
	// Fields after the command name, which is in parentheses and may contain spaces.
	s := string(b)
	f := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	if len(f) < 13 {
		return 0, 0, fmt.Errorf("short /proc/%d/stat", pid)
	}
	ut, _ := strconv.ParseInt(f[11], 10, 64)
	st, _ := strconv.ParseInt(f[12], 10, 64)
	cpu := time.Duration(ut+st) * time.Second / clockTicks

	fh, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return cpu, 0, err
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "VmRSS:"); ok {
			kb, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
			return cpu, kb << 10, nil
		}
	}
	return cpu, 0, nil
}
