package graph

import (
	"testing"

	"github.com/uk0/silk/geom"
)

func TestResizeCarryStopsAtLocalCoordinateBoundary(t *testing.T) {
	for _, localRoot := range []bool{false, true} {
		box := NewRectItem()
		box.SetBounds(10, 20, 60, 60)
		box.SetLocalCoord(localRoot)
		panel := NewRectItem()
		panel.SetBounds(15, 25, 30, 30)
		panel.SetLocalCoord(true)
		panel.SetParent(box)
		leaf := NewRectItem()
		leaf.SetBounds(5, 5, 10, 10)
		leaf.SetParent(panel)
		before := []geom.Rect{box.Bounds1(), panel.Bounds1(), leaf.Bounds1()}
		oldX, oldY := leaf.MapToScene(leaf.Pos())
		cmd := NewResizeCommand()
		cmd.AddItem(box, geom.Rect{X: 17, Y: 23, Width: 53, Height: 57})
		appendResizeCarriedChildren(cmd)
		wantCount := 2
		if localRoot {
			wantCount = 1
		}
		if cmd.Count() != wantCount {
			t.Fatalf("localRoot=%v: records=%d, want %d", localRoot, cmd.Count(), wantCount)
		}
		for pass := 0; pass < 2; pass++ {
			cmd.Redo()
			if leaf.Bounds1() != before[2] {
				t.Fatal("resize modified coordinates below a local-coordinate boundary")
			}
			if x, y := leaf.MapToScene(leaf.Pos()); x != oldX+7 || y != oldY+3 {
				t.Fatalf("localRoot=%v: leaf scene position=(%v,%v), want (%v,%v)", localRoot, x, y, oldX+7, oldY+3)
			}
			cmd.Undo()
			for i, item := range []IItem{box, panel, leaf} {
				if item.Bounds1() != before[i] {
					t.Fatalf("undo did not restore item %d", i)
				}
			}
		}
	}
}

type resizeManagedBox struct{ RectItem }

func (box *resizeManagedBox) ManagesChildGeometry() bool { return true }
func (box *resizeManagedBox) Layout() {
	for _, child := range box.Children() {
		child.SetBounds(box.X()+2, box.Y()+3, box.Width()-4, 7)
	}
}

func TestResizeCarryDoesNotRecordLayoutManagedChildren(t *testing.T) {
	for _, nested := range []bool{false, true} {
		managed := new(resizeManagedBox)
		managed.Init(managed)
		managed.SetBounds(15, 25, 30, 30)
		leaf := NewRectItem()
		leaf.SetParent(managed)
		managed.Layout()
		var box IItem = managed
		if nested {
			box = NewRectItem()
			box.SetBounds(10, 20, 60, 60)
			managed.SetParent(box)
		}
		before := []geom.Rect{box.Bounds1(), managed.Bounds1(), leaf.Bounds1()}
		target := box.Bounds1()
		target.X += 7
		target.Y += 3
		target.Width -= 7
		target.Height -= 3
		cmd := NewResizeCommand()
		cmd.AddItem(box, target)
		appendResizeCarriedChildren(cmd)
		wantCount := 1
		if nested {
			wantCount = 2
		}
		if cmd.Count() != wantCount {
			t.Fatalf("nested=%v: records=%d, want %d (no managed leaf record)", nested, cmd.Count(), wantCount)
		}
		for pass := 0; pass < 2; pass++ {
			cmd.Redo()
			if leaf.X() != before[2].X+7 || leaf.Y() != before[2].Y+3 || leaf.Width() != managed.Width()-4 {
				t.Fatalf("nested=%v: layout child moved twice or was stranded: %v", nested, leaf.Bounds1())
			}
			cmd.Undo()
			for i, item := range []IItem{box, managed, leaf} {
				if item.Bounds1() != before[i] {
					t.Fatalf("nested=%v: undo did not restore item %d: %v, want %v", nested, i, item.Bounds1(), before[i])
				}
			}
		}
	}
}

func TestResizeCarryPreservesExplicitChildResizeAndLockedOffsets(t *testing.T) {
	box := NewRectItem()
	box.SetBounds(10, 10, 60, 60)
	child := NewRectItem()
	child.SetBounds(20, 20, 30, 30)
	child.SetParent(box)
	leaf := NewRectItem()
	leaf.SetBounds(25, 25, 10, 10)
	leaf.SetLockPos(true)
	leaf.SetLockSize(true)
	leaf.SetParent(child)
	before := []geom.Rect{box.Bounds1(), child.Bounds1(), leaf.Bounds1()}
	cmd := NewResizeCommand()
	cmd.AddItem(box, geom.Rect{X: 20, Y: 30, Width: 50, Height: 40})
	cmd.AddItem(child, geom.Rect{X: 24, Y: 26, Width: 26, Height: 24})
	appendResizeCarriedChildren(cmd)
	if cmd.Count() != 3 {
		t.Fatalf("child recorded twice or locked descendant left behind: %d records", cmd.Count())
	}
	for pass := 0; pass < 2; pass++ {
		cmd.Redo()
		if child.X() != 24 || child.Y() != 26 || leaf.X() != 29 || leaf.Y() != 31 || leaf.Width() != 10 || leaf.Height() != 10 {
			t.Fatal("descendant did not follow its explicit parent resize exactly once")
		}
		cmd.Undo()
		for i, item := range []IItem{box, child, leaf} {
			if item.Bounds1() != before[i] {
				t.Fatalf("undo did not restore item %d", i)
			}
		}
	}
}
