package store

import (
	"reflect"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// TestSettingsGroups checks that every field of the server settings but
// UpdatedAt belongs to a group, so that none goes unstored.
func TestSettingsGroups(t *testing.T) {
	var s core.ServerSettings
	grouped := map[uintptr]bool{}
	for _, g := range settingsGroups(&s) {
		grouped[reflect.ValueOf(g.value).Pointer()] = true
	}
	v := reflect.ValueOf(&s).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Name != "UpdatedAt" && !grouped[v.Field(i).Addr().Pointer()] {
			t.Errorf("field %s is in no group", f.Name)
		}
	}
}
