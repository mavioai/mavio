package proto_test

import (
	"testing"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

const validID = "0190f5a4-3b2c-7d1e-8f00-123456789012"

func TestValidationRules(t *testing.T) {
	spec := func(mut func(*libraryv1.LibrarySpec)) *libraryv1.LibrarySpec {
		s := libraryv1.LibrarySpec_builder{
			Name:  proto.String("Movies"),
			Kind:  libraryv1.LibraryKind_LIBRARY_KIND_MOVIES.Enum(),
			Paths: []string{"/media/movies"},
		}.Build()
		if mut != nil {
			mut(s)
		}
		return s
	}

	tests := []struct {
		name  string
		msg   proto.Message
		valid bool
	}{
		{"get item", libraryv1.GetItemRequest_builder{Id: proto.String(validID)}.Build(), true},
		{"get item without id", &libraryv1.GetItemRequest{}, false},
		{"get item bad id", libraryv1.GetItemRequest_builder{Id: proto.String("42")}.Build(), false},
		{"create library", libraryv1.CreateLibraryRequest_builder{Spec: spec(nil)}.Build(), true},
		{"create library without spec", &libraryv1.CreateLibraryRequest{}, false},
		{"library without kind", libraryv1.CreateLibraryRequest_builder{Spec: spec(func(s *libraryv1.LibrarySpec) { s.ClearKind() })}.Build(), false},
		{"library without paths", libraryv1.CreateLibraryRequest_builder{Spec: spec(func(s *libraryv1.LibrarySpec) { s.SetPaths(nil) })}.Build(), false},
		{"library bad language", libraryv1.CreateLibraryRequest_builder{Spec: spec(func(s *libraryv1.LibrarySpec) { s.SetPreferredLanguage("eng") })}.Build(), false},
		{"list items", libraryv1.ListItemsRequest_builder{Limit: proto.Int32(50)}.Build(), true},
		{"list items limit too large", libraryv1.ListItemsRequest_builder{Limit: proto.Int32(1001)}.Build(), false},
		{"list items bad parent", libraryv1.ListItemsRequest_builder{ParentId: proto.String("x")}.Build(), false},
		{"list items unspecified kind", libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_UNSPECIFIED}}.Build(), false},
		{"create user", userv1.CreateUserRequest_builder{Name: proto.String("alice"), Password: proto.String("pw")}.Build(), true},
		{"create user without name", userv1.CreateUserRequest_builder{Password: proto.String("pw")}.Build(), false},
		{"user data rating", userv1.UpdateUserDataRequest_builder{ItemId: proto.String(validID), Rating: proto.Float64(11)}.Build(), false},
		{"preferences language", userv1.UpdatePreferencesRequest_builder{Preferences: userv1.UserPreferences_builder{AudioLanguages: []string{"en"}}.Build()}.Build(), false},
	}
	for _, tt := range tests {
		err := protovalidate.Validate(tt.msg)
		if (err == nil) != tt.valid {
			t.Errorf("%s: Validate() = %v, want valid=%v", tt.name, err, tt.valid)
		}
	}
}
