package parse

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	agoPattern       = regexp.MustCompile(`(?i)\bago`)
	todayPattern     = regexp.MustCompile(`(?i)\btoday(?:[\s,]+(?:at)?\s*|[\s,]*|$)`)
	tomorrowPattern  = regexp.MustCompile(`(?i)\btomorrow(?:[\s,]+(?:at)?\s*|[\s,]*|$)`)
	yesterdayPattern = regexp.MustCompile(`(?i)\byesterday(?:[\s,]+(?:at)?\s*|[\s,]*|$)`)
	weekdayPattern   = regexp.MustCompile(`(?i)\b(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+at\s+`)
	noYearPattern    = regexp.MustCompile(`^(\d{1,2}-\d{1,2})(\s|$)`)
	noYearPattern2   = regexp.MustCompile(`^(\d{1,2}\s+\w{3})\s+(\d{1,2}\:\d{1,2}.*)$`) // 1 Jan 10:30
	agoPart          = regexp.MustCompile(`\s*?([\d\.]+)\s*?([^\d\s\.]+)\s*?`)
	allDigits        = regexp.MustCompile(`^\d+$`)
)

var weekdays = map[string]time.Weekday{
	"monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday, "sunday": time.Sunday,
}

// ErrDate is returned for strings that are not dates.
var ErrDate = errors.New("not a date")

// TimeAgo reads a time relative to now such as "2 hours 1 day ago",
// "3 wks, 2 d" or "now".
func TimeAgo(now time.Time, s string) (time.Time, error) {
	s = strings.ToLower(s)
	if strings.Contains(s, "now") {
		return now, nil
	}
	s = strings.NewReplacer(",", "", "ago", "", "and", "").Replace(s)
	var ago time.Duration
	for _, m := range agoPart.FindAllStringSubmatch(s, -1) {
		v, err := Float(m[1])
		if err != nil {
			return time.Time{}, err
		}
		unit := m[2]
		var d time.Duration
		switch {
		case strings.Contains(unit, "sec") || unit == "s":
			d = time.Second
		case strings.Contains(unit, "min") || unit == "m":
			d = time.Minute
		case strings.Contains(unit, "hour") || strings.Contains(unit, "hr") || unit == "h":
			d = time.Hour
		case strings.Contains(unit, "day") || unit == "d":
			d = 24 * time.Hour
		case strings.Contains(unit, "week") || strings.Contains(unit, "wk") || unit == "w":
			d = 7 * 24 * time.Hour
		case strings.Contains(unit, "month") || unit == "mo":
			d = 30 * 24 * time.Hour
		case strings.Contains(unit, "year") || unit == "y":
			d = 365 * 24 * time.Hour
		default:
			return time.Time{}, fmt.Errorf("%w: unknown unit %q", ErrDate, unit)
		}
		ago += time.Duration(v * float64(d))
	}
	return now.Add(-ago), nil
}

// Fuzzy reads a date or time anywhere in s with FuzzyTime; format "UK"
// reads numeric dates day first.
func Fuzzy(now time.Time, s, format string) (time.Time, error) {
	order := MonthFirst
	if format == "UK" {
		order = DayFirst
	}
	t, ok := FuzzyTime(now, s, order)
	if !ok {
		return time.Time{}, fmt.Errorf("%w: %q", ErrDate, s)
	}
	return t, nil
}

// Unknown reads a date in any form indexers write them: RFC 1123, a Unix
// timestamp, "now", a time ago, "today", "yesterday" or "tomorrow" with
// an optional time, a day of the week "at" a time, or anything Fuzzy
// reads, also without its year. format is that of Fuzzy.
func Unknown(now time.Time, s, format string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 02 Jan 2006 15:04:05 -07"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if allDigits.MatchString(s) {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(n, 0).In(now.Location()), nil
		}
	}
	if strings.Contains(strings.ToLower(s), "now") {
		return now, nil
	}
	if agoPattern.MatchString(s) {
		return TimeAgo(now, s)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, rel := range []struct {
		re   *regexp.Regexp
		days int
	}{{todayPattern, 0}, {yesterdayPattern, -1}, {tomorrowPattern, 1}} {
		if m := rel.re.FindString(s); m != "" {
			return today.AddDate(0, 0, rel.days).Add(timeOfDay(strings.ReplaceAll(s, m, ""))), nil
		}
	}
	if m := weekdayPattern.FindStringSubmatch(s); m != nil {
		d := today.Add(timeOfDay(strings.ReplaceAll(s, m[0], "")))
		for d.Weekday() != weekdays[strings.ToLower(m[1])] {
			d = d.AddDate(0, 0, -1)
		}
		return d, nil
	}
	if m := noYearPattern.FindStringSubmatch(s); m != nil {
		s = strings.ReplaceAll(s, m[1], strconv.Itoa(now.Year())+"-"+m[1])
	}
	if m := noYearPattern2.FindStringSubmatch(s); m != nil {
		s = m[1] + " " + strconv.Itoa(now.Year()) + " " + m[2]
	}
	return Fuzzy(now, s, format)
}

// timeOfDay reads a time such as "14:22" or "2:30 PM"; zero for none.
func timeOfDay(s string) time.Duration {
	d, _ := findTime([]rune(strings.TrimSpace(s)), MonthFirst, nil)
	return d
}
