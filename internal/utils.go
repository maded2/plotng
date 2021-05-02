package internal

import (
	"fmt"
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
