package agents

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func DueScheduledAt(job Job, now time.Time) (time.Time, bool, error) {
	if !job.Enabled {
		return time.Time{}, false, nil
	}
	location := time.Local
	if job.Schedule.Timezone != "" {
		loaded, err := time.LoadLocation(job.Schedule.Timezone)
		if err != nil {
			return time.Time{}, false, err
		}
		location = loaded
	}
	localNow := now.In(location).Truncate(time.Minute)
	if job.Schedule.Every != "" {
		duration, err := time.ParseDuration(job.Schedule.Every)
		if err != nil {
			return time.Time{}, false, err
		}
		if duration <= 0 {
			return time.Time{}, false, fmt.Errorf("schedule.every must be positive")
		}
		return localNow.Truncate(duration), true, nil
	}
	if job.Schedule.Cron != "" {
		scheduled, ok, err := previousCronTime(job.Schedule.Cron, localNow)
		if err != nil || !ok {
			return time.Time{}, ok, err
		}
		return scheduled, true, nil
	}
	return time.Time{}, false, nil
}

// NextScheduledAt returns the next eligible schedule slot strictly after now.
// A daemon still has to be running at that time for the job to execute.
func NextScheduledAt(job Job, now time.Time) (time.Time, bool, error) {
	if !job.Enabled {
		return time.Time{}, false, nil
	}
	location := time.Local
	if job.Schedule.Timezone != "" {
		loaded, err := time.LoadLocation(job.Schedule.Timezone)
		if err != nil {
			return time.Time{}, false, err
		}
		location = loaded
	}
	localNow := now.In(location)
	if job.Schedule.Every != "" {
		duration, err := time.ParseDuration(job.Schedule.Every)
		if err != nil {
			return time.Time{}, false, err
		}
		if duration <= 0 {
			return time.Time{}, false, fmt.Errorf("schedule.every must be positive")
		}
		return localNow.Truncate(duration).Add(duration), true, nil
	}
	if job.Schedule.Cron != "" {
		return nextCronTime(job.Schedule.Cron, localNow)
	}
	return time.Time{}, false, nil
}

func validateCronExpression(expression string) error {
	if strings.HasPrefix(expression, "@") {
		switch expression {
		case "@hourly", "@daily", "@weekly":
			return nil
		default:
			return fmt.Errorf("unsupported cron alias %q", expression)
		}
	}
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return fmt.Errorf("must have five fields: minute hour day-of-month month day-of-week")
	}
	ranges := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for index, field := range fields {
		if err := validateCronField(field, ranges[index][0], ranges[index][1], index == 4); err != nil {
			return err
		}
	}
	return nil
}

func previousCronTime(expression string, now time.Time) (time.Time, bool, error) {
	expression = expandCronAlias(expression)
	if err := validateCronExpression(expression); err != nil {
		return time.Time{}, false, err
	}
	fields := strings.Fields(expression)
	for candidate := now; candidate.After(now.AddDate(0, 0, -366)); candidate = candidate.Add(-time.Minute) {
		if cronMatches(fields, candidate) {
			return candidate, true, nil
		}
	}
	return time.Time{}, false, nil
}

func nextCronTime(expression string, now time.Time) (time.Time, bool, error) {
	expression = expandCronAlias(expression)
	if err := validateCronExpression(expression); err != nil {
		return time.Time{}, false, err
	}
	fields := strings.Fields(expression)
	first := now.Truncate(time.Minute).Add(time.Minute)
	limit := first.AddDate(1, 0, 1)
	for candidate := first; candidate.Before(limit); candidate = candidate.Add(time.Minute) {
		if cronMatches(fields, candidate) {
			return candidate, true, nil
		}
	}
	return time.Time{}, false, nil
}

func expandCronAlias(expression string) string {
	switch expression {
	case "@hourly":
		return "0 * * * *"
	case "@daily":
		return "0 0 * * *"
	case "@weekly":
		return "0 0 * * 0"
	default:
		return expression
	}
}

func cronMatches(fields []string, candidate time.Time) bool {
	values := []int{
		candidate.Minute(),
		candidate.Hour(),
		candidate.Day(),
		int(candidate.Month()),
		int(candidate.Weekday()),
	}
	for index, field := range fields {
		if !cronFieldMatches(field, values[index], index == 4) {
			return false
		}
	}
	return true
}

func validateCronField(field string, min int, max int, weekday bool) error {
	if field == "*" {
		return nil
	}
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return fmt.Errorf("cron field %q has an empty value", field)
		}
		value, ok := weekdayNameValue(part)
		if ok && !weekday {
			return fmt.Errorf("cron field %q must use numbers outside the day-of-week field", field)
		}
		if !ok {
			parsed, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("cron field %q must use *, comma-separated numbers, or weekday names", field)
			}
			value = parsed
		}
		if value < min || value > max {
			return fmt.Errorf("cron field %q is outside %d-%d", field, min, max)
		}
	}
	return nil
}

func cronFieldMatches(field string, value int, weekday bool) bool {
	if field == "*" {
		return true
	}
	for _, part := range strings.Split(field, ",") {
		partValue, ok := weekdayNameValue(part)
		if !ok {
			parsed, err := strconv.Atoi(part)
			if err != nil {
				return false
			}
			partValue = parsed
		}
		if weekday && partValue == 7 {
			partValue = 0
		}
		if partValue == value {
			return true
		}
	}
	return false
}

func weekdayNameValue(value string) (int, bool) {
	switch strings.ToUpper(value) {
	case "SUN":
		return 0, true
	case "MON":
		return 1, true
	case "TUE":
		return 2, true
	case "WED":
		return 3, true
	case "THU":
		return 4, true
	case "FRI":
		return 5, true
	case "SAT":
		return 6, true
	default:
		return 0, false
	}
}
