package agent

import (
	"strconv"
	"strings"
)

// Bu dosya, atıf arka ucu yetenek ölçümünün platformdan bağımsız saf
// yardımcılarını taşır (Linux karar mantığı attrsource_linux.go'da, ama
// bu ayrıştırıcılar her platformda derlenir ve test edilir).

// parseKernelVersion, "6.8.0-51-generic" / "5.10.0" / "4.19" gibi bir
// uname/osrelease dizesinden major.minor çıkarır.
func parseKernelVersion(s string) (major, minor int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	// ilk '-', '+' veya boşluktan öncesini al ("6.8.0-51-generic" → "6.8.0")
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	var err error
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, false
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

// kernelAtLeast, (major, minor)'ın en az (wantMaj, wantMin) olup olmadığını söyler.
func kernelAtLeast(major, minor, wantMaj, wantMin int) bool {
	if major != wantMaj {
		return major > wantMaj
	}
	return minor >= wantMin
}

// parseCapEff, /proc/self/status içeriğindeki "CapEff:" satırından effective
// capability bit maskesini çözer.
func parseCapEff(procStatus string) (uint64, bool) {
	for _, line := range strings.Split(procStatus, "\n") {
		rest, found := strings.CutPrefix(line, "CapEff:")
		if !found {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(rest), 16, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// capBit, bir capability maskesinde verilen bitin set olup olmadığını söyler.
func capBit(mask uint64, bit uint) bool { return mask&(uint64(1)<<bit) != 0 }
