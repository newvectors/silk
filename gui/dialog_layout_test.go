package gui

import "testing"

func TestDialogRespectsExplicitModalSize(t *testing.T) {
	d := NewDialog("Project Settings", nil)
	d.SetContent(NewLabel("small content"))
	d.AddButton("Close", DialogOK)
	d.SetSize(560, 420)
	if w, h := d.dialogSize(); w != 560 || h != 420 {
		t.Fatalf("explicit size replaced: %gx%g", w, h)
	}
	d.SetBounds(1, 2, 720, 540)
	if w, h := d.dialogSize(); w != 720 || h != 540 {
		t.Fatalf("explicit bounds replaced: %gx%g", w, h)
	}
}

func TestDialogFooterButtonsAreInsetAndDoNotOverlapContent(t *testing.T) {
	d := NewDialog("Settings", nil)
	d.SetContent(NewLabel("content"))
	one := d.AddButton("Cancel", DialogCancel)
	two := d.AddButton("Close", DialogOK)
	d.SetSize(560, 420)
	d.Layout()
	for _, b := range []*Button{one, two} {
		if b.Width() < 88 || b.Height() < 32 || b.Y() < 8 {
			t.Fatalf("unusable click target: %gx%g at %g", b.Width(), b.Height(), b.Y())
		}
	}
	if two.X()+two.Width() != d.Width()-24 || one.X()+one.Width()+8 != two.X() {
		t.Fatal("footer not right-aligned with spacing")
	}
	if d.Content().Y()+d.Content().Height() > d.btnBox.Y()-16 {
		t.Fatal("content overlaps footer")
	}
	one.SetVisible(false)
	d.Layout()
	if two.X()+two.Width() != d.Width()-24 {
		t.Fatal("hidden button left trailing space")
	}
}

func TestDialogMultipleActionsExpandBeforeShowing(t *testing.T) {
	d := NewDialog("Actions", nil)
	for _, text := range []string{"One", "Two", "Three", "Four"} {
		d.AddButton(text, DialogCancel)
	}
	w, h := d.dialogSize()
	d.SetSize(w, h)
	d.Layout()
	for i, b := range d.buttons {
		if b.X() < 24 || i > 0 && d.buttons[i-1].X()+d.buttons[i-1].Width()+8 > b.X() {
			t.Fatal("dialog action targets overlap or hit the edge")
		}
	}
}
