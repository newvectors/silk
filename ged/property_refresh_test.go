package ged

import "testing"

type propertyRefreshProbe struct {
	refresh func()
}

func (*propertyRefreshProbe) Clear(interface{})                       {}
func (*propertyRefreshProbe) Bind([]interface{}, string, interface{}) {}
func (p *propertyRefreshProbe) RefreshValues()                        { p.refresh() }

func TestResizeAndHistoryRefreshPropertyValues(t *testing.T) {
	view, scene := newResizeSnapView(t, 5, true)
	btn := addFakeAt(t, scene, "btn", 10, 10, 25, 7)
	view.Selection().Add(btn)
	var width, height float64
	calls := 0
	view.SetPropertyView(&propertyRefreshProbe{refresh: func() { calls++; width, height = btn.Size() }})
	dragBottomRightHandle(t, view, 34.5, 16.5, 60, 40)
	if width != btn.Width() || height != btn.Height() || width == 25 {
		t.Fatal("resize left stale property values")
	}
	view.Undo()
	if width != btn.Width() || height != btn.Height() || width != 25 {
		t.Fatal("undo left stale property values")
	}
	view.Redo()
	if width != btn.Width() || height != btn.Height() || calls != 3 {
		t.Fatal("redo did not refresh properties")
	}
}
