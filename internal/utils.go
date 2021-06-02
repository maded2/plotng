package internal

import (
	"fmt"
	"math"
	"time"
)

func DurationString(d time.Duration) string {
	hour := d / time.Hour
	d = d - hour*(60*60*1e9)
	mins := d / time.Minute
	d = d - mins*(60*1e9)
	secs := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", hour, mins, secs)
}

func SpaceString(s uint64) string {
	if s == math.MaxUint64 {
		return "???"
	}
	if s > 1000 * GB {
		return fmt.Sprintf("%0.2f TiB", float64(s) / float64(TB))
	}
	return fmt.Sprintf("%d GiB", s / GB)
}
