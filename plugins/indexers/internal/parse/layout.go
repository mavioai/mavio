package parse

import (
	"fmt"
	"strings"
	"time"
)

// Layout reads a date with a layout, which Cardigann definitions give
// either in Go's form ("2006-01-02 15:04:05 -07:00") or in .NET's
// ("yyyy-MM-dd HH:mm:ss zzz"). Dates without an offset are in now's
// location; a date without a year is this year's, a time without a date
// today's.
func Layout(now time.Time, s, layout string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// .NET formats mostly name their years, hours or days with y, h or d;
	// either form is tried when the first fails.
	layouts := []string{layout, DotNetLayout(layout)}
	if strings.ContainsAny(strings.ToLower(layout), "yhd") {
		layouts[0], layouts[1] = layouts[1], layouts[0]
	}
	var err error
	for _, l := range layouts {
		var t time.Time
		if t, err = time.ParseInLocation(l, s, now.Location()); err == nil {
			return completeDate(now, t, l), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: %q as %q: %w", ErrDate, s, layout, err)
}

// completeDate gives t, parsed with a Go layout, now's year when the
// layout has none and now's date when it has neither month nor day.
func completeDate(now, t time.Time, layout string) time.Time {
	format := func(y int, m time.Month, d int) string {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format(layout)
	}
	// The pairs fall on the same weekday, which layouts may name.
	base := format(2001, time.February, 3)
	hasYear := base != format(2007, time.February, 3)
	hasMonth := base != format(2001, time.March, 3)
	hasDay := base != format(2001, time.February, 10)
	y, m, d := t.Date()
	if !hasYear {
		y = now.Year()
	}
	if !hasMonth && !hasDay {
		_, m, d = now.Date()
	}
	return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// DotNetLayout converts a .NET custom date format such as
// "ddd, dd MMM yyyy HH':'mm':'ss zzz" into a Go layout.
func DotNetLayout(format string) string {
	var b strings.Builder
	r := []rune(format)
	for i := 0; i < len(r); {
		c := r[i]
		n := 1
		for i+n < len(r) && r[i+n] == c {
			n++
		}
		switch c {
		case '\'', '"':
			end := i + 1
			for end < len(r) && r[end] != c {
				end++
			}
			b.WriteString(string(r[i+1 : min(end, len(r))]))
			i = end + 1
			continue
		case '\\':
			if i+1 < len(r) {
				b.WriteRune(r[i+1])
			}
			i += 2
			continue
		case 'y':
			b.WriteString(pick(n, "06", "06", "2006"))
		case 'M':
			b.WriteString(pick(n, "1", "01", "Jan", "January"))
		case 'd':
			b.WriteString(pick(n, "2", "02", "Mon", "Monday"))
		case 'h':
			b.WriteString(pick(n, "3", "03"))
		case 'H':
			b.WriteString("15")
		case 'm':
			b.WriteString(pick(n, "4", "04"))
		case 's':
			b.WriteString(pick(n, "5", "05"))
		case 'f', 'F':
			// Go's fractions include their separator.
			digit := "0"
			if c == 'F' {
				digit = "9"
			}
			s := b.String()
			sep := "."
			if strings.HasSuffix(s, ".") || strings.HasSuffix(s, ",") {
				sep = s[len(s)-1:]
				b.Reset()
				b.WriteString(s[:len(s)-1])
			}
			b.WriteString(sep + strings.Repeat(digit, n))
		case 't':
			b.WriteString("PM")
		case 'z':
			b.WriteString(pick(n, "-07", "-07", "-07:00"))
		case 'K':
			b.WriteString("Z07:00")
		default:
			b.WriteString(strings.Repeat(string(c), n))
		}
		i += n
	}
	return b.String()
}

// pick returns the form for a run of n letters, the last for longer runs.
func pick(n int, forms ...string) string {
	return forms[min(n, len(forms))-1]
}
