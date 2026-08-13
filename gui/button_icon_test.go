package gui

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/uk0/silk/paint"
)

// iconIdentity names the icon a paint.Icon actually is. Icon() != nil proves
// nothing about which icon a button got: paint.LoadIcon never returns nil — ""
// yields the empty "air" icon and a name no theme resolves yields the red-X
// "image-missing" one — so every name, right or wrong, satisfies it. The
// concrete icon is a fmt.Stringer over the name it was loaded under, and that
// name is the one thing that tells the right icon from the wrong one with no
// icon files installed.
func iconIdentity(ico paint.Icon) string {
	switch v := ico.(type) {
	case nil:
		return "<nil>"
	case fmt.Stringer:
		if name := v.String(); name != "" {
			return name
		}
		return "<air>"
	default:
		return fmt.Sprintf("<unnamed %T>", ico)
	}
}

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
// loads the icon of that name and is readable back, a second name replaces the
// first, "" clears both, and an icon set directly leaves no name behind — a
// nameless icon that reported one would have the design file save, and codegen
// emit, an icon the user never chose.
//
// The names used are "build" and "debug", two of the affordances paint resolves
// procedurally (paint/icon_fallbacks.go). They are the icons observable with no
// icon theme installed: an unresolvable name like "save" comes back as the
// shared "image-missing" red X, which every wrong name comes back as too, so
// nothing about it could be asserted.
func TestButtonIconName(t *testing.T) {
	btn := NewButton()

	btn.SetIconName("build")
	if got := btn.IconName(); got != "build" {
		t.Errorf(`IconName() after SetIconName("build") = %q, want "build"`, got)
	}
	if got := iconIdentity(btn.Icon()); got != "build" {
		t.Errorf(`Icon() after SetIconName("build") is the %s icon, want the build one`, got)
	}

	// The name is recorded after the load, so a load that reads the field
	// instead of its argument keeps serving the icon of the previous name
	// while IconName() already reports the new one.
	btn.SetIconName("debug")
	if got := btn.IconName(); got != "debug" {
		t.Errorf(`IconName() after SetIconName("debug") = %q, want "debug"`, got)
	}
	if got := iconIdentity(btn.Icon()); got != "debug" {
		t.Errorf(`Icon() after SetIconName("debug") is the %s icon, want the debug one; the button reports an icon it is not carrying`, got)
	}

	btn.SetIconName("")
	if got := btn.IconName(); got != "" {
		t.Errorf(`IconName() after SetIconName("") = %q, want ""`, got)
	}
	if ico := btn.Icon(); ico != nil {
		t.Errorf(`Icon() after SetIconName("") = %v, want nil`, ico)
	}

	btn.SetIcon(LoadIcon("build"))
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
