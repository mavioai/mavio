package parse

import (
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

// The date and time patterns of Sergey Stoyan's DateTimeRoutines, which
// Prowlarr uses to find a date or time anywhere in a string.

// DateOrder is the order of day and month in numeric dates.
type DateOrder int

const (
	// MonthFirst reads 12/31/2007 as December 31.
	MonthFirst DateOrder = iota
	// DayFirst reads 31/12/2007 as December 31.
	DayFirst
)

func mustCompile(expr string, opts regexp2.RegexOptions) *regexp2.Regexp {
	re := regexp2.MustCompile(expr, opts)
	re.MatchTimeout = time.Second
	return re
}

const (
	timeAMPM   = `(?:\s*(?'ampm'AM|am|PM|pm))?`
	monthNames = `(?'month'Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[uarychilestmbro]*`
)

var (
	// The time zone suffixes of each order: US dates know US zones.
	timeZones = map[DateOrder]string{
		MonthFirst: `(?:\s*(?'time_zone'UTC|GMT|CST|EST))?`,
		DayFirst:   `(?:\s*(?'time_zone'UTC|GMT))?`,
	}
	// Times after a date: hh:mm:ss <UTC offset>, then [h]h:mm[:ss] [AM/PM]
	// [zone]; times before or within a date: [h]h:mm:ss [AM/PM] [zone];
	// times anywhere: hh:mm:ss <UTC offset>, then [h]h:mm[:ss] [AM/PM] [zone].
	timeAfterDateOffset = mustCompile(`(?<=^\s*,?\s+|^\s*at\s*|^\s*[T\-]\s*)(?'hour'\d{2})\s*:\s*(?'minute'\d{2})\s*:\s*(?'second'\d{2})\s+(?'offset_sign'[\+\-])(?'offset_hh'\d{2}):?(?'offset_mm'\d{2})(?=$|[^\d\w])`, 0)
	timeAfterDate       = timePatterns(`(?<=^\s*,?\s+|^\s*at\s*|^\s*[T\-]\s*)(?'hour'\d{1,2})\s*:\s*(?'minute'\d{2})\s*(?::\s*(?'second'\d{2}))?`+timeAMPM, `(?=$|[^\d\w])`)
	timeAroundDate      = timePatterns(`(?<=^|[^\d])(?'hour'\d{1,2})\s*:\s*(?'minute'\d{2})\s*(?::\s*(?'second'\d{2}))?`+timeAMPM, `(?=$|[\s,]+)`)
	timeAnywhereOffset  = mustCompile(`(?<=^|\s+|\s*T\s*)(?'hour'\d{2})\s*:\s*(?'minute'\d{2})\s*:\s*(?'second'\d{2})\s+(?'offset_sign'[\+\-])(?'offset_hh'\d{2}):?(?'offset_mm'\d{2})?(?=$|[^\d\w])`, 0)
	timeAnywhere        = timePatterns(`(?<=^|\s+|\s*T\s*)(?'hour'\d{1,2})\s*:\s*(?'minute'\d{2})\s*(?::\s*(?'second'\d{2}))?`+timeAMPM, `(?=$|[^\d\w])`)

	// Dates: dd/mm/yy, [yy]yy-mm-dd, month dd yyyy, dd month [yy]yy, yyyy
	// month dd, month dd hh:mm:ss MDT|UTC yyyy and month dd [yyyy].
	dateNumeric = mustCompile(`(?<=^|[^\d])(?'day'\d{1,2})\s*(?'separator'[\\/\.])+\s*(?'month'\d{1,2})\s*\'separator'+\s*(?'year'\d{2}|\d{4})(?=$|[^\d])`, regexp2.IgnoreCase)
	dateISO     = mustCompile(`(?<=^|[^\d])(?'year'\d{2}|\d{4})\s*(?'separator'[\-])\s*(?'month'\d{1,2})\s*\'separator'+\s*(?'day'\d{1,2})(?=$|[^\d])`, regexp2.IgnoreCase)
	dateNamed   = []*regexp2.Regexp{
		mustCompile(`(?:^|[^\d\w])`+monthNames+`\s+(?'day'\d{1,2})(?:-?st|-?th|-?rd|-?nd)?\s*,?\s*(?'year'\d{4})(?=$|[^\d\w])`, regexp2.IgnoreCase),
		mustCompile(`(?:^|[^\d\w:])(?'day'\d{1,2})(?:-?st\s+|-?th\s+|-?rd\s+|-?nd\s+|-|\s+)`+monthNames+`(?:\s*,?\s*|-)'?(?'year'\d{2}|\d{4})(?=$|[^\d\w])`, regexp2.IgnoreCase),
		mustCompile(`(?:^|[^\d\w])(?'year'\d{4})\s+`+monthNames+`\s+(?'day'\d{1,2})(?:-?st|-?th|-?rd|-?nd)?(?=$|[^\d\w])`, regexp2.IgnoreCase),
		mustCompile(`(?:^|[^\d\w])`+monthNames+`\s+(?'day'\d{1,2})\s+\d{2}\:\d{2}\:\d{2}\s+(?:MDT|UTC)\s+(?'year'\d{4})(?=$|[^\d\w])`, regexp2.IgnoreCase),
		mustCompile(`(?:^|[^\d\w])`+monthNames+`\s+(?'day'\d{1,2})(?:-?st|-?th|-?rd|-?nd)?(?:\s*,?\s*(?'year'\d{4}))?(?=$|[^\d\w])`, regexp2.IgnoreCase),
	}
)

func timePatterns(expr, tail string) map[DateOrder]*regexp2.Regexp {
	m := map[DateOrder]*regexp2.Regexp{}
	for order, zone := range timeZones {
		m[order] = mustCompile(expr+zone+tail, 0)
	}
	return m
}

var months = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"may": time.May, "jun": time.June, "jul": time.July, "aug": time.August,
	"sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

// span is where a date was found, in runes.
type span struct{ index, length int }

// FuzzyTime finds a date, a time or both anywhere in s, as in
// "Last Update: 18:16 11 Feb '08" or "Expires Apr 1st, 2007, 14:16:32",
// with the date's order of day and month when it is numeric. A time
// without a date is today's, a date without a year this year's, both as of
// now and in its location, where the result is; UTC offsets the string
// gives are ignored.
func FuzzyTime(now time.Time, s string, order DateOrder) (time.Time, bool) {
	r := []rune(s)
	date, at, dateOK := findDate(now, r, order)
	var clock time.Duration
	var timeOK bool
	if dateOK {
		clock, timeOK = findTime(r, order, &at)
	} else {
		clock, timeOK = findTime(r, order, nil)
		if !timeOK {
			return time.Time{}, false
		}
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	if timeOK {
		date = date.Add(clock)
	}
	return date, true
}

// findTime finds a time of day, around the date at when there is one.
func findTime(r []rune, order DateOrder, at *span) (time.Duration, bool) {
	var m *regexp2.Match
	if at != nil {
		after := r[at.index+at.length:]
		m = match(timeAfterDateOffset, after)
		if m == nil {
			m = match(timeAfterDate[order], after)
		}
		if m == nil {
			m = match(timeAroundDate[order], r[:at.index])
		}
		if m == nil {
			m = match(timeAroundDate[order], r[at.index:at.index+at.length])
		}
	} else {
		m = match(timeAnywhereOffset, r)
		if m == nil {
			m = match(timeAnywhere[order], r)
		}
	}
	if m == nil {
		return 0, false
	}
	hour, _ := strconv.Atoi(group(m, "hour"))
	minute, _ := strconv.Atoi(group(m, "minute"))
	second := 0
	if s := group(m, "second"); s != "" {
		second, _ = strconv.Atoi(s)
	}
	if hour > 23 || minute > 59 || second > 59 {
		return 0, false
	}
	switch ampm := strings.ToUpper(group(m, "ampm")); {
	case ampm == "PM" && hour < 12:
		hour += 12
	case ampm == "AM" && hour == 12:
		hour = 0
	}
	return time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second, true
}

// findDate finds a date in now's location and where it is.
func findDate(now time.Time, r []rune, order DateOrder) (time.Time, span, bool) {
	if len(r) == 0 {
		return time.Time{}, span{}, false
	}
	if m := match(dateNumeric, r); m != nil {
		day, month := atoi(group(m, "day")), atoi(group(m, "month"))
		if order == MonthFirst {
			day, month = month, day
		}
		d, ok := makeDate(now, atoi(group(m, "year")), month, day)
		return d, span{m.Index, m.Length}, ok
	}
	if m := match(dateISO, r); m != nil {
		d, ok := makeDate(now, atoi(group(m, "year")), atoi(group(m, "month")), atoi(group(m, "day")))
		return d, span{m.Index, m.Length}, ok
	}
	for _, re := range dateNamed {
		m := match(re, r)
		if m == nil {
			continue
		}
		year := now.Year()
		if y := group(m, "year"); y != "" {
			year = atoi(y)
		}
		month, ok := months[strings.ToLower(group(m, "month"))]
		if !ok {
			return time.Time{}, span{}, false
		}
		d, ok := makeDate(now, year, int(month), atoi(group(m, "day")))
		return d, span{m.Index, m.Length}, ok
	}
	return time.Time{}, span{}, false
}

// makeDate makes a valid date, reading two-digit years up to 30 as this
// century's.
func makeDate(now time.Time, year, month, day int) (time.Time, bool) {
	switch {
	case year >= 100 && year < 1000:
		return time.Time{}, false
	case year < 100 && year > 30:
		year += 1900
	case year < 100:
		year += 2000
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
	if d.Year() != year || int(d.Month()) != month || d.Day() != day {
		return time.Time{}, false
	}
	return d, true
}

func match(re *regexp2.Regexp, r []rune) *regexp2.Match {
	m, err := re.FindRunesMatch(r)
	if err != nil {
		return nil
	}
	return m
}

func group(m *regexp2.Match, name string) string {
	if g := m.GroupByName(name); g != nil && len(g.Captures) > 0 {
		return g.String()
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
