package organize

import (
	"errors"
	"testing"
)

// The cases follow the validators of Radarr's FileNameValidation, which
// has no fixture of its own.
func TestValidateMovieFormat(t *testing.T) {
	cases := []struct {
		format string
		want   error
	}{
		{"{Movie Title} ({Release Year}) {Quality Full}", nil},
		{"{Movie.CleanTitle}.{Release.Year}", nil},
		{"{Original Title}", nil},
		{"{Movie Title} {Original Filename}", nil},
		{"{Movie Title}", ErrMissingTokens},
		{"{Release Year}", ErrMissingTokens},
		{"", ErrEmptyFormat},
		{"{Movie Title} ({Release Year})\x00", ErrIllegalCharacters},
	}
	for _, c := range cases {
		if got := ValidateMovieFormat(c.format); !errors.Is(got, c.want) {
			t.Errorf("ValidateMovieFormat(%q) = %v, want = %v", c.format, got, c.want)
		}
	}
}

func TestValidateMovieFolderFormat(t *testing.T) {
	cases := []struct {
		format string
		want   error
	}{
		{"{Movie Title} ({Release Year})", nil},
		{"{Movie CollectionThe}/{Movie TitleThe}", nil},
		{"{Movie Title} {Quality Full}", ErrDeprecatedFolderTokens},
		{"{Movie Title} [{MediaInfo VideoCodec}]", ErrDeprecatedFolderTokens},
		{"{Release Year}", ErrMissingTokens},
		{" ", ErrEmptyFormat},
	}
	for _, c := range cases {
		if got := ValidateMovieFolderFormat(c.format); !errors.Is(got, c.want) {
			t.Errorf("ValidateMovieFolderFormat(%q) = %v, want = %v", c.format, got, c.want)
		}
	}
}

func TestNamingValidate(t *testing.T) {
	if err := DefaultNaming().Validate(); err != nil {
		t.Errorf("DefaultNaming().Validate() = %v, want = nil", err)
	}
	n := DefaultNaming()
	n.ColonReplacement = "colon"
	if err := n.Validate(); !errors.Is(err, ErrInvalidColonReplacement) {
		t.Errorf("Validate() = %v, want = %v", err, ErrInvalidColonReplacement)
	}
}
