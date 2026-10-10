package release

import "strings"

var romanNumerals = []struct {
	value  int
	symbol string
}{
	{1000, "M"},
	{900, "CM"},
	{500, "D"},
	{400, "CD"},
	{100, "C"},
	{90, "XC"},
	{50, "L"},
	{40, "XL"},
	{10, "X"},
	{9, "IX"},
	{5, "V"},
	{4, "IV"},
	{1, "I"},
}

// RomanNumeral returns n, from 1 to 3999, as a Roman numeral.
func RomanNumeral(n int) string {
	var b strings.Builder
	for _, r := range romanNumerals {
		for n >= r.value {
			b.WriteString(r.symbol)
			n -= r.value
		}
	}
	return b.String()
}

// ParseRomanNumeral returns the value of a Roman numeral, ignoring case.
func ParseRomanNumeral(s string) (int, bool) {
	rest := strings.ToUpper(s)
	n := 0
	for _, r := range romanNumerals {
		for strings.HasPrefix(rest, r.symbol) {
			n += r.value
			rest = rest[len(r.symbol):]
		}
	}
	if rest != "" || n == 0 || RomanNumeral(n) != strings.ToUpper(s) {
		return 0, false
	}
	return n, true
}
