package util

import "time"

func DaysBetween(start_nano, end_nano int64) int {
	startTime := time.Unix(0, start_nano)
	endTime := time.Unix(0, end_nano)
	return int(endTime.Sub(startTime).Hours() / 24)
}
