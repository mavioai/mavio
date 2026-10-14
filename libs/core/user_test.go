package core

import (
	"errors"
	"testing"
)

func TestUserKinds(t *testing.T) {
	id := NewID()
	tests := []struct {
		name string
		u    User
		ok   bool
	}{
		{"member", User{ID: id, Name: "a", Kind: UserMember}, true},
		{"administrator", User{ID: id, Name: "a", Kind: UserAdmin}, true},
		{"member through a plugin", User{ID: id, Name: "a", Kind: UserMember, AuthProvider: "ldap"}, true},
		{"no kind", User{ID: id, Name: "a"}, false},
		{"managed", User{ID: id, Name: "a", Kind: UserManaged, Restriction: RestrictionTeen}, true},
		{"managed through a plugin", User{ID: id, Name: "a", Kind: UserManaged, AuthProvider: "ldap"}, false},
		{"restricted member", User{ID: id, Name: "a", Kind: UserMember, Restriction: RestrictionTeen}, false},
		{"unknown profile", User{ID: id, Name: "a", Kind: UserManaged, Restriction: "toddler"}, false},
		{"Mavio account", User{ID: id, Name: "a", Kind: UserMember, AccountID: "acct"}, true},
		{"Mavio account through a plugin", User{ID: id, Name: "a", Kind: UserMember, AccountID: "acct", AuthProvider: "ldap"}, false},
	}
	for _, tt := range tests {
		if err := tt.u.Validate(); (err == nil) != tt.ok || err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestRestrictionProfiles(t *testing.T) {
	score := func(n int) *int { return &n }
	item := func(rating *int) *Item { return &Item{Kind: KindMovie, ParentalRating: rating} }
	tests := []struct {
		name    string
		profile RestrictionProfile
		access  LibraryAccess
		item    *Item
		want    bool
	}{
		{"no profile, R", RestrictionNone, LibraryAccess{}, item(score(17)), true},
		{"no profile, unrated", RestrictionNone, LibraryAccess{}, item(nil), true},
		{"younger child, G", RestrictionYoungerChild, LibraryAccess{}, item(score(0)), true},
		{"younger child, TV-Y7", RestrictionYoungerChild, LibraryAccess{}, item(score(7)), false},
		{"older child, PG", RestrictionOlderChild, LibraryAccess{}, item(score(10)), true},
		{"older child, PG-13", RestrictionOlderChild, LibraryAccess{}, item(score(13)), false},
		{"teen, TV-14", RestrictionTeen, LibraryAccess{}, item(score(14)), true},
		{"teen, R", RestrictionTeen, LibraryAccess{}, item(score(17)), false},
		{"teen, unrated", RestrictionTeen, LibraryAccess{}, item(nil), false},
		{"teen under a lower limit", RestrictionTeen, LibraryAccess{ContentLimits: ContentLimits{MaxParentalRating: score(10)}}, item(score(13)), false},
	}
	for _, tt := range tests {
		kind := UserMember
		if tt.profile != RestrictionNone {
			kind = UserManaged
		}
		u := User{ID: NewID(), Name: "kid", Kind: kind, Restriction: tt.profile, Access: tt.access}
		if got := u.CanAccess(tt.item); got != tt.want {
			t.Errorf("%s: CanAccess = %v, want = %v", tt.name, got, tt.want)
		}
	}
}
