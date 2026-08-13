package gui

import "testing"

// buttonBoxNames reads back what the box will actually submit: every child
// button's ExtraData, in layout order. onBtnClick passes exactly this string to
// the SigSubmit callback, so the payload — not the visible caption — is what
// decides whether a click means "@ok" or "@cancel".
func buttonBoxNames(t *testing.T, box *ButtonBox) []string {
	t.Helper()
	var out []string
	for i, iw := range box.Items() {
		btn, ok := iw.(*Button)
		if !ok {
			t.Fatalf("child %d is a %T, want *Button", i, iw)
		}
		name, ok := btn.ExtraData().(string)
		if !ok {
			t.Fatalf("child %d carries ExtraData %#v, want the button name", i, btn.ExtraData())
		}
		out = append(out, name)
	}
	return out
}

// TestButtonBoxSetButtonsSpec pins the designer-facing spelling of the button
// list. Two failures it catches: an untrimmed spec, which submits " @ok "
// instead of "@ok" and so matches nothing in the caller's switch; and a spec
// that adds to the box instead of replacing it, which leaves the previous row
// of buttons on screen after the property is edited.
func TestButtonBoxSetButtonsSpec(t *testing.T) {
	box := NewButtonBox()

	box.SetButtonsSpec(" @ok , @cancel ")
	if got := box.ButtonsSpec(); got != "@ok,@cancel" {
		t.Errorf("ButtonsSpec() = %q, want %q", got, "@ok,@cancel")
	}
	names := buttonBoxNames(t, box)
	if len(names) != 2 || names[0] != "@ok" || names[1] != "@cancel" {
		t.Errorf("buttons = %q, want [@ok @cancel]", names)
	}

	box.SetButtonsSpec("@yes")
	if got := box.ButtonsSpec(); got != "@yes" {
		t.Errorf("ButtonsSpec() after replacement = %q, want %q", got, "@yes")
	}
	names = buttonBoxNames(t, box)
	if len(names) != 1 || names[0] != "@yes" {
		t.Errorf("buttons after replacement = %q, want [@yes]", names)
	}
}
