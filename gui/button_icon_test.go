package gui

import (
	"reflect"
	"testing"
)

// recordingPropertyList implements core.IPropertyList by recording what
// EnumProperties hands it, so the property sheet's view of a widget can be
// asserted without a GL surface. Same shape ged/morph.go's setter probe uses.
type recordingPropertyList struct {
	entries []recordedProperty
}

type recordedProperty struct {
	id  string
	get interface{}
	set interface{}
}

func (r *recordingPropertyList) AddProperty(id string, get, set interface{}) {
	r.entries = append(r.entries, recordedProperty{id: id, get: get, set: set})
}

func (r *recordingPropertyList) find(id string) (recordedProperty, bool) {
	for _, e := range r.entries {
		if e.id == id {
			return e, true
		}
	}
	return recordedProperty{}, false
}

// TestButtonIconName covers the name<->icon pair the designer edits: a name
// loads an icon and is readable back, "" clears both, and an icon set directly
// leaves no name behind — a nameless icon that reported one would have the
// design file save, and codegen emit, an icon the user never chose.
func TestButtonIconName(t *testing.T) {
	btn := NewButton()

	btn.SetIconName("save")
	if got := btn.IconName(); got != "save" {
		t.Errorf(`IconName() after SetIconName("save") = %q, want "save"`, got)
	}
	if btn.Icon() == nil {
		t.Error(`Icon() after SetIconName("save") is nil, want the loaded icon`)
	}

	btn.SetIconName("")
	if got := btn.IconName(); got != "" {
		t.Errorf(`IconName() after SetIconName("") = %q, want ""`, got)
	}
	if ico := btn.Icon(); ico != nil {
		t.Errorf(`Icon() after SetIconName("") = %v, want nil`, ico)
	}

	btn.SetIcon(LoadIcon("save"))
	if got := btn.IconName(); got != "" {
		t.Errorf(`IconName() after SetIcon() = %q, want "" (a directly set icon has no name)`, got)
	}
}

// TestButtonEnumPropertiesIcon: the icon is reachable from the property sheet
// and from the design file's "props" capture, both of which read the widget
// through EnumProperties. A string setter is what makes it round-trip.
func TestButtonEnumPropertiesIcon(t *testing.T) {
	rec := new(recordingPropertyList)
	NewButton().EnumProperties(rec)

	e, ok := rec.find("图标")
	if !ok {
		t.Fatalf(`Button.EnumProperties has no "图标" property; got %v`, propertyIDs(rec))
	}
	if e.get == nil {
		t.Error(`"图标" has no getter`)
	}
	st := reflect.TypeOf(e.set)
	if st == nil || st.Kind() != reflect.Func || st.NumIn() != 1 || st.NumOut() != 0 ||
		st.In(0).Kind() != reflect.String {
		t.Fatalf(`"图标" setter has type %v, want func(string)`, st)
	}
}

func propertyIDs(r *recordingPropertyList) []string {
	ids := make([]string, 0, len(r.entries))
	for _, e := range r.entries {
		ids = append(ids, e.id)
	}
	return ids
}
