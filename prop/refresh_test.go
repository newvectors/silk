package prop

import "testing"

func TestRefreshValuesPreservesControlsAndDoesNotCreateCommands(t *testing.T) {
	a, b := "25", "35"
	first := &fakeObj{enum: func(list IPropertyList) {
		list.AddProperty("width", func() string { return a }, func(s string) { a = s })
	}}
	second := &fakeObj{enum: func(list IPropertyList) {
		list.AddProperty("width", func() string { return b }, func(s string) { b = s })
	}}
	owner := &recordingOwner{}
	sheet := bindSheet([]interface{}{first, second}, owner)
	item := sheet.propMap["width"]
	control := item.Control().(*TextEdit)
	if control.Text() != "" {
		t.Fatal("mixed property was not empty")
	}
	a = "35" // Simulate a resize outside the property editor.
	sheet.RefreshValues()
	if item.Control() != control || control.Text() != "35" {
		t.Fatalf("refresh replaced the control or left stale text: %q", control.Text())
	}
	if len(owner.cmds) != 0 {
		t.Fatal("refresh created an edit command")
	}
}
