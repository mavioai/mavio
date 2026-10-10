// Package parse reads the numbers, sizes, IDs and dates indexers print,
// as Prowlarr's ParseUtil, DateTimeUtil and DateTimeRoutines do: numbers
// with either decimal separator, sizes such as "1,023.4 MB", IMDb IDs with
// or without their "tt", and dates in whatever form a site writes them,
// from RFC 1123 to "Yesterday at 14:22", "3 hours ago" or
// "Sept 30th 2008". The patterns keep their .NET syntax, run by
// github.com/dlclark/regexp2.
//
// Functions that read relative dates take the time to read them against,
// whose location is that of dates without an offset.
package parse
