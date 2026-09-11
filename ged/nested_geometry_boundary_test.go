package ged

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/gui"
)

func freeContainerFixture(t *testing.T) (*GedScene, *FakeWidget, *FakeWidget) {
	t.Helper()
	scene := NewGedScene()
	scene.SetSize(200, 150)
	box, err := NewFakeWidgetFromFactory("gui.ScrollArea")
	if err != nil {
		t.Fatal(err)
	}
	box.SetWidgetName("Free")
	box.SetBounds(30, 35, 80, 60)
	box.SetParent(scene)
	leaf, err := NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatal(err)
	}
	leaf.SetWidgetName("Leaf")
	leaf.SetBounds(35, 40, 20, 8)
	leaf.SetParent(box)
	return scene, box, leaf
}
func TestFreeContainerPreviewUsesParentLocalCoordinates(t *testing.T) {
	scene, box, _ := freeContainerFixture(t)
	runtime := box.Generate()
	child := runtime.Children()[0]
	want := gui.MmToPixelZ(5)
	if child.X() != want || child.Y() != want {
		t.Fatalf("preview child=(%g,%g), want local (%g,%g); absolute coordinates double-offset child", child.X(), child.Y(), want, want)
	}
	for _, node := range []*decl.Node{buildSceneDeclNode(scene), mustDeclFromDesign(t, scene.SaveDesign())} {
		_, idx, err := node.BuildWithIndex()
		if err != nil {
			t.Fatal(err)
		}
		got := idx["Leaf"].(gui.IWidget)
		wantDecl := roundPx(5)
		if math.Abs(got.X()-wantDecl) > 1 || math.Abs(got.Y()-wantDecl) > 1 {
			t.Fatalf("decl child=(%g,%g), want local %g", got.X(), got.Y(), wantDecl)
		}
	}
}
func TestManagedParentCarriesFreeContainerGrandchildren(t *testing.T) {
	_, free, leaf := freeContainerFixture(t)
	managed, _ := NewFakeWidgetFromFactory("gui.GridLayout")
	managed.layoutManaged = true
	managed.SetBounds(10, 10, 100, 100)
	free.SetParent(managed)
	free.layoutCell = layoutPlacement{Row: 0, Column: 0, RowSpan: 1, ColumnSpan: 1, Valid: true}
	beforeDX, beforeDY := leaf.X()-free.X(), leaf.Y()-free.Y()
	managed.Layout()
	if leaf.X()-free.X() != beforeDX || leaf.Y()-free.Y() != beforeDY {
		t.Fatalf("grandchild floated: local offset=(%g,%g), want (%g,%g)", leaf.X()-free.X(), leaf.Y()-free.Y(), beforeDX, beforeDY)
	}
	managed.SetPos(50, 60)
	if leaf.X()-free.X() != beforeDX || leaf.Y()-free.Y() != beforeDY {
		t.Fatal("moving managed parent stranded free grandchild")
	}
}

func TestFreeContainerPropertyOriginCarriesSubtree(t *testing.T) {
	_, box, leaf := freeContainerFixture(t)
	inner, _ := NewFakeWidgetFromFactory("gui.VBox")
	inner.SetBounds(40, 55, 30, 20)
	inner.SetParent(box)
	grandchild, _ := NewFakeWidgetFromFactory("gui.Edit")
	grandchild.SetBounds(45, 60, 20, 7)
	grandchild.SetParent(inner)
	box.setDesignX(50)
	box.setDesignY(60)
	if leaf.X() != 55 || leaf.Y() != 65 || inner.X() != 60 || inner.Y() != 80 || grandchild.X() != 65 || grandchild.Y() != 85 {
		t.Fatalf("property movement stranded descendants: leaf%v inner%v grandchild%v", leaf.Bounds1(), inner.Bounds1(), grandchild.Bounds1())
	}
	box.setDesignX(30)
	box.setDesignY(35)
	if grandchild.X() != 45 || grandchild.Y() != 60 {
		t.Fatal("property undo failed to restore subtree")
	}
}
func TestGeneratedFreeContainerUsesLocalGeometry(t *testing.T) {
	if testing.Short() {
		t.Skip("generated Go integration")
	}
	scene, _, _ := freeContainerFixture(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "nested.go")
	testFile := filepath.Join(dir, "nested_test.go")
	code := scene.GenerateCode(CodeGenOptions{PackageName: "ui", TypeName: "NestedFree"})
	if err := os.WriteFile(src, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	fixture := `package ui
import "testing"
func TestChildLocal(t *testing.T){u:=NewNestedFree();if u.Leaf.X()!=19||u.Leaf.Y()!=19{t.Fatalf("double offset: %g,%g",u.Leaf.X(),u.Leaf.Y())}}
`
	if err := os.WriteFile(testFile, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "test", src, testFile).CombinedOutput(); err != nil {
		t.Fatalf("generated runtime: %v\n%s", err, out)
	}
}

func TestLegacyFreeLayoutsStayFreeAfterPreviewRelayout(t *testing.T) {
	for _, kind := range []LayoutKind{LayoutHorizontal, LayoutVertical, LayoutGrid, LayoutForm} {
		t.Run(string(kind), func(t *testing.T) {
			scene, old, leaf := freeContainerFixture(t)
			box, err := NewFakeWidgetFromFactory(string(kind))
			if err != nil {
				t.Fatal(err)
			}
			box.SetWidgetName("Free")
			box.SetBounds1(old.Bounds1())
			old.SetParent(nil)
			box.SetParent(scene)
			leaf.SetParent(box)
			inner, _ := NewFakeWidgetFromFactory("gui.VBox")
			inner.SetWidgetName("Inner")
			inner.SetBounds(45, 55, 25, 20)
			inner.SetParent(box)
			grandchild, _ := NewFakeWidgetFromFactory("gui.Edit")
			grandchild.SetWidgetName("Grandchild")
			grandchild.SetBounds(50, 60, 15, 7)
			grandchild.SetParent(inner)
			check := func(root gui.IWidget, unit float64) {
				t.Helper()
				manager := root.(interface {
					LayoutEnabled() bool
					Layout()
				})
				if manager.LayoutEnabled() {
					t.Fatal("legacy free layout silently enabled")
				}
				root.SetSize(root.Width()*1.5, root.Height()*1.5)
				manager.Layout()
				children := root.Children()
				if len(children) != 2 {
					t.Fatalf("children=%d", len(children))
				}
				first, innerRuntime := children[0], children[1]
				if math.Abs(first.X()-5*unit) > 1 || math.Abs(first.Y()-5*unit) > 1 {
					t.Fatalf("child moved after layout: %g,%g", first.X(), first.Y())
				}
				innerRuntime.(interface{ Layout() }).Layout()
				nested := innerRuntime.Children()[0]
				if math.Abs(nested.X()-5*unit) > 1 || math.Abs(nested.Y()-5*unit) > 1 {
					t.Fatalf("grandchild moved after layout: %g,%g", nested.X(), nested.Y())
				}
			}
			check(box.Generate(), gui.ScreenDpmm())
			for _, node := range []*decl.Node{buildSceneDeclNode(scene), mustDeclFromDesign(t, scene.SaveDesign())} {
				_, idx, err := node.BuildWithIndex()
				if err != nil {
					t.Fatal(err)
				}
				check(idx["Free"].(gui.IWidget), 96.0/25.4)
			}
		})
	}
}
