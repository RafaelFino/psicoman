package service

import "time"

func parseTimeString(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try without timezone
		t, _ = time.Parse("2006-01-02T15:04:05", s)
	}
	return t.UTC()
}
