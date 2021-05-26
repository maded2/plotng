package internal

import (
	"fmt"
	"math"
	"time"
)

func durationString(d time.Duration) string {
	hour := d / time.Hour
	d = d - hour*(60*60*1e9)
	mins := d / time.Minute
	d = d - mins*(60*1e9)
	secs := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", hour, mins, secs)
}

func spaceString(s uint64) string {
	if s == math.MaxUint64 {
		return "???"
	}
	if s > 1000*gb {
		return fmt.Sprintf("%0.2f TiB", float64(s)/float64(tb))
	}
	return fmt.Sprintf("%d GiB", s/gb)
}
