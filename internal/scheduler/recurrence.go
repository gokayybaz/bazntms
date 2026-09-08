package scheduler

// Yineleme belirteci (spec) ayrıştırma + bir sonraki koşum hesabı. Biçimler:
//
//	interval:<dk>            — her <dk> dakikada (referanstan)
//	daily:<HH:MM>            — her gün yerel <HH:MM>
//	weekly:<gün>:<HH:MM>     — gün ∈ mon,tue,wed,thu,fri,sat,sun
//	monthly:<1..31>:<HH:MM>  — ayın <n>. günü (ay o güne sahip değilse ay sonu)

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// NextRun, spec için `after`'dan sonraki ilk koşum zamanını döndürür (yerel).
func NextRun(spec string, after time.Time) (time.Time, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), ":")
	switch parts[0] {
	case "interval":
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("interval:<dk> bekleniyordu")
		}
		m, err := strconv.Atoi(parts[1])
		if err != nil || m <= 0 {
			return time.Time{}, fmt.Errorf("geçersiz aralık: %q", parts[1])
		}
		return after.Add(time.Duration(m) * time.Minute), nil

	case "daily":
		hh, mm, err := hm(parts, 1)
		if err != nil {
			return time.Time{}, err
		}
		cand := time.Date(after.Year(), after.Month(), after.Day(), hh, mm, 0, 0, after.Location())
		if !cand.After(after) {
			cand = cand.AddDate(0, 0, 1)
		}
		return cand, nil

	case "weekly":
		if len(parts) != 4 {
			return time.Time{}, fmt.Errorf("weekly:<gün>:<HH:MM> bekleniyordu")
		}
		wd, ok := weekdays[parts[1]]
		if !ok {
			return time.Time{}, fmt.Errorf("geçersiz gün: %q", parts[1])
		}
		hh, mm, err := hm(parts, 2)
		if err != nil {
			return time.Time{}, err
		}
		cand := time.Date(after.Year(), after.Month(), after.Day(), hh, mm, 0, 0, after.Location())
		for i := 0; i < 8; i++ {
			if cand.Weekday() == wd && cand.After(after) {
				return cand, nil
			}
			cand = cand.AddDate(0, 0, 1)
		}
		return cand, nil

	case "monthly":
		if len(parts) != 4 {
			return time.Time{}, fmt.Errorf("monthly:<1..31>:<HH:MM> bekleniyordu")
		}
		dom, err := strconv.Atoi(parts[1])
		if err != nil || dom < 1 || dom > 31 {
			return time.Time{}, fmt.Errorf("geçersiz ayın günü: %q", parts[1])
		}
		hh, mm, err := hm(parts, 2)
		if err != nil {
			return time.Time{}, err
		}
		for i := 0; i < 3; i++ {
			y, mo := after.Year(), after.Month()
			mo = time.Month(int(mo) + i)
			for int(mo) > 12 {
				mo -= 12
				y++
			}
			d := dom
			if last := daysInMonth(y, mo); d > last {
				d = last
			}
			cand := time.Date(y, mo, d, hh, mm, 0, 0, after.Location())
			if cand.After(after) {
				return cand, nil
			}
		}
		return time.Time{}, fmt.Errorf("monthly: sonraki koşum bulunamadı")
	}
	return time.Time{}, fmt.Errorf("bilinmeyen yineleme: %q", spec)
}

func hm(parts []string, i int) (int, int, error) {
	if len(parts) < i+2 {
		return 0, 0, fmt.Errorf("HH:MM eksik")
	}
	hh, e1 := strconv.Atoi(parts[i])
	mm, e2 := strconv.Atoi(parts[i+1])
	if e1 != nil || e2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, fmt.Errorf("geçersiz HH:MM")
	}
	return hh, mm, nil
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
