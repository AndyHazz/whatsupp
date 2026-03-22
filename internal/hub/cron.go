package hub

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// nextCronTime calculates the next time a 5-field cron expression fires after `after`.
// Format: minute hour day-of-month month day-of-week
// Supports: numbers, *, */N, and comma-separated lists.
func nextCronTime(spec string, after time.Time) (time.Time, error) {
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron: expected 5 fields, got %d", len(fields))
	}

	minuteSet, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron minute: %w", err)
	}
	hourSet, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron hour: %w", err)
	}
	domSet, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron day-of-month: %w", err)
	}
	monthSet, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron month: %w", err)
	}
	dowSet, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron day-of-week: %w", err)
	}

	// Start from the next minute after `after`
	t := after.Truncate(time.Minute).Add(time.Minute)

	// Search up to 366 days ahead
	limit := after.Add(366 * 24 * time.Hour)
	for t.Before(limit) {
		if monthSet[int(t.Month())] &&
			domSet[t.Day()] &&
			dowSet[int(t.Weekday())] &&
			hourSet[t.Hour()] &&
			minuteSet[t.Minute()] {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("cron: no match found within 366 days")
}

// parseCronField parses a single cron field into a set of valid values.
func parseCronField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") {
			// Step: */N or M/N
			pieces := strings.SplitN(part, "/", 2)
			step, err := strconv.Atoi(pieces[1])
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step: %s", part)
			}
			start := min
			if pieces[0] != "*" {
				start, err = strconv.Atoi(pieces[0])
				if err != nil {
					return nil, fmt.Errorf("invalid range start: %s", part)
				}
			}
			for i := start; i <= max; i += step {
				result[i] = true
			}
		} else if part == "*" {
			for i := min; i <= max; i++ {
				result[i] = true
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			if v < min || v > max {
				return nil, fmt.Errorf("value %d out of range [%d,%d]", v, min, max)
			}
			result[v] = true
		}
	}

	return result, nil
}
