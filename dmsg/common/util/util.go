package util

import "time"

func DaysBetween(start, end int64) int {
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	return int(endTime.Sub(startTime).Hours() / 24)
}
