package subtitles

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// TranslationStatistics sums up the translations of the subtitle
// history, as Lingarr's statistics.
type TranslationStatistics struct {
	Files, Cues, Characters   int64
	InputTokens, OutputTokens int64
	Cost                      float64
	// ByTranslator, ByLanguagePair ("<source>:<target>") and ByMedia
	// ("movie" or "episode") count files.
	ByTranslator, ByLanguagePair, ByMedia map[string]int64
	// Daily counts the files of each of the last days, oldest first.
	Daily []TranslationDay
}

// TranslationDay is the number of files translated on a day.
type TranslationDay struct {
	Date  string
	Files int64
}

// TranslationStatistics sums up the translations of the subtitle history,
// counting the files of each of the last days in the server's time zone.
func (r *Runtime) TranslationStatistics(ctx context.Context, days int) (TranslationStatistics, error) {
	st := TranslationStatistics{ByTranslator: map[string]int64{}, ByLanguagePair: map[string]int64{}, ByMedia: map[string]int64{}}
	today := r.now().Local()
	first := time.Date(today.Year(), today.Month(), today.Day()-days+1, 0, 0, 0, 0, time.Local)
	daily := map[string]int64{}
	q := core.SubtitleHistoryQuery{Actions: []core.SubtitleAction{core.SubtitleTranslated}, Limit: core.MaxPageSize}
	for {
		page, err := r.Store.SubtitleHistory().List(ctx, q)
		if err != nil {
			return st, err
		}
		for _, e := range page.Items {
			st.Files++
			media := "movie"
			if !e.ParentID.IsZero() {
				media = "episode"
			}
			st.ByMedia[media]++
			if day := e.Date.Local(); days > 0 && !day.Before(first) {
				daily[day.Format(time.DateOnly)]++
			}
			rec := e.Translation
			if rec == nil {
				continue
			}
			st.Cues += int64(rec.Cues)
			st.Characters += rec.Characters
			st.InputTokens += rec.InputTokens
			st.OutputTokens += rec.OutputTokens
			st.Cost += rec.Cost
			if len(rec.Translators) > 0 {
				st.ByTranslator[rec.Translators[0]]++
			}
			if rec.SourceLanguage != "" {
				st.ByLanguagePair[rec.SourceLanguage+":"+e.Language.Language]++
			}
		}
		q.Offset += len(page.Items)
		if len(page.Items) == 0 || q.Offset >= page.Total {
			break
		}
	}
	for d := range days {
		date := first.AddDate(0, 0, d).Format(time.DateOnly)
		st.Daily = append(st.Daily, TranslationDay{Date: date, Files: daily[date]})
	}
	return st, nil
}
