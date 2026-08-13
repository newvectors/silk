package main

import (
	"strings"
	"testing"

	"github.com/uk0/silk/ged"
	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// designerCanvas installs a headless canvas as the frame's current document,
// which is the only way the 布局 menu's handlers can reach one: they all open
// with requireSelection, and that walks gui.DefaultFrame().CurrentDocView().
// The frame carries a status bar because setStatus writes there and nowhere
// else — without one every sentence a handler produces is dropped on the floor.
func designerCanvas(t *testing.T) *ged.GedView {
	t.Helper()
	f := gui.NewFrame()
	f.SetStatusBar(gui.NewStatusBar())
	gui.SetDefaultFrame(f)
	gv := ged.NewGedView()
	f.SuggestDocDock().AddView(gv)
	if cur := currentGedView(); cur != gv {
		t.Fatalf("the frame hands back %v as the current document, not the canvas", cur)
	}
	return gv
}

// placeWidget drops a widget through the same AddCommand the canvas's drop
// handler uses, then gives it a name and a row of its own so a VBox/HBox has
// something to sort the children by.
func placeWidget(t *testing.T, gv *ged.GedView, factory, name string, y float64) *ged.FakeWidget {
	t.Helper()
	dropWidget(t, gv, factory)
	kids := gv.Scene().Children()
	fw, ok := kids[len(kids)-1].(*ged.FakeWidget)
	if !ok {
		t.Fatalf("dropping %s put a %T on the canvas", factory, kids[len(kids)-1])
	}
	fw.SetWidgetName(name)
	fw.SetBounds(10, y, 30, 8)
	return fw
}

func selectWidgets(gv *ged.GedView, items ...*ged.FakeWidget) {
	sel := gv.Selection()
	sel.Clear()
	for _, it := range items {
		sel.Add(it)
	}
}

// namesOf labels a set of items for a failure message. The identity checks
// below are what the assertions turn on; this is only there to say what turned
// up instead.
func namesOf(items []graph.IItem) string {
	var out []string
	for _, it := range items {
		if fw, ok := it.(*ged.FakeWidget); ok {
			out = append(out, fw.WidgetName())
		} else {
			out = append(out, "?")
		}
	}
	return "[" + strings.Join(out, " ") + "]"
}

// hasItems checks membership by identity, never by count: a lay-out that
// dropped the selected widgets and built fresh ones in their place would hold
// the right number of children and still have thrown the user's work away.
func hasItems(t *testing.T, what string, got []graph.IItem, want ...*ged.FakeWidget) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s holds %d items %s, want %d", what, len(got), namesOf(got), len(want))
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == graph.IItem(w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not hold the original %q; it holds %s", what, w.WidgetName(), namesOf(got))
		}
	}
}

// soleContainer returns the scene's single top-level child, which is what a
// lay-out leaves behind, and pins the container type it was asked for.
func soleContainer(t *testing.T, gv *ged.GedView, factory string) *ged.FakeWidget {
	t.Helper()
	top := gv.Scene().Children()
	if len(top) != 1 {
		t.Fatalf("scene top-level = %d %s, want 1 (the new container)", len(top), namesOf(top))
	}
	box, ok := top[0].(*ged.FakeWidget)
	if !ok {
		t.Fatalf("scene top-level item is a %T, not a widget", top[0])
	}
	if got := box.WidgetFactoryName(); got != factory {
		t.Fatalf("container is a %q, want %q", got, factory)
	}
	return box
}

// runLayoutMenuItem clicks one 布局 menu item and reports both halves of what
// the user has to go on: the sentence left in the status bar, and how many
// commands the canvas actually pushed. Clearing the bar first is what keeps a
// handler that says nothing at all from passing on the previous message.
func runLayoutMenuItem(t *testing.T, gv *ged.GedView, click func()) (said string, pushed int) {
	t.Helper()
	sb := gui.DefaultFrame().StatusBar()
	if sb == nil {
		t.Fatal("the frame has no status bar; nothing the handlers say can be read back")
	}
	sb.ClearMessage()
	stack := gv.Scene().UndoStack()
	before := stack.Current()
	click()
	return sb.Message(), stack.Current() - before
}

// wantDeclined: the canvas refused the selection, so the status bar has to
// carry the reason. Nothing was pushed, so there is no other trace of the click
// anywhere — a menu item that reports success here is indistinguishable from
// one that worked.
func wantDeclined(t *testing.T, what, said string, pushed int, reason string) {
	t.Helper()
	if said != reason {
		t.Errorf("%s told the user %q, want the decline %q", what, said, reason)
	}
	if pushed != 0 {
		t.Errorf("%s pushed %d command(s) while declining; the canvas and the status bar disagree", what, pushed)
	}
}

// wantApplied is the other half: the same handler on a selection it accepts has
// to say something different, and leave the one step Ctrl+Z undoes.
func wantApplied(t *testing.T, what, said string, pushed int, report string) {
	t.Helper()
	if said != report {
		t.Errorf("%s told the user %q, want %q", what, said, report)
	}
	if pushed != 1 {
		t.Errorf("%s pushed %d command(s), want the single step Ctrl+Z undoes", what, pushed)
	}
}

// TestApplyHBoxLayoutDeclinesALoneWidgetOutLoud drives the case the canvas
// refuses in silence: GedView.LayOutSelection returns without a word when fewer
// than two selected items are layout-eligible, and one selected widget is
// never two. The handler tells the two outcomes apart by watching the undo
// stack; without that comparison 应用水平布局 reports the layout it did not
// apply, over a canvas the widgets are still lying loose on.
func TestApplyHBoxLayoutDeclinesALoneWidgetOutLoud(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)

	selectWidgets(gv, a)
	said, pushed := runLayoutMenuItem(t, gv, applyHBoxLayout)
	wantDeclined(t, "应用水平布局 on a lone widget", said, pushed, "请至少选择2个可布局的控件来应用布局")
	hasItems(t, "the scene after the declined layout", gv.Scene().Children(), a, b)

	// The accepted click has to read differently, or the status bar says the
	// same thing whether the command ran or not.
	selectWidgets(gv, a, b)
	said, pushed = runLayoutMenuItem(t, gv, applyHBoxLayout)
	wantApplied(t, "应用水平布局 on two widgets", said, pushed, "已应用水平布局 (HBox)")
	hasItems(t, "the HBox", soleContainer(t, gv, "gui.HBox").Children(), a, b)
}

// TestApplyVBoxLayoutDeclinesALockedSelectionOutLoud is the second way a
// selection declines, and the one the count in the menu's own enablement rule
// cannot catch: position-locked widgets drop out of the lay-out, so two locked
// widgets are as empty a selection as one widget is. 应用垂直布局 stays enabled
// and has to say why it did nothing.
func TestApplyVBoxLayoutDeclinesALockedSelectionOutLoud(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)
	a.SetLockPos(true)
	b.SetLockPos(true)
	selectWidgets(gv, a, b)

	said, pushed := runLayoutMenuItem(t, gv, applyVBoxLayout)
	wantDeclined(t, "应用垂直布局 on a position-locked selection", said, pushed, "请至少选择2个可布局的控件来应用布局")
	hasItems(t, "the scene after the declined layout", gv.Scene().Children(), a, b)

	a.SetLockPos(false)
	b.SetLockPos(false)
	said, pushed = runLayoutMenuItem(t, gv, applyVBoxLayout)
	wantApplied(t, "应用垂直布局 once the widgets are unlocked", said, pushed, "已应用垂直布局 (VBox)")
	hasItems(t, "the VBox", soleContainer(t, gv, "gui.VBox").Children(), a, b)
}

// TestBreakLayoutDeclinesAPlainWidgetOutLoud: 打破布局 has nothing to lift out
// of a widget that holds no children, so the canvas builds an empty command and
// drops it. Reporting 布局已打破 there names a container the user never had.
func TestBreakLayoutDeclinesAPlainWidgetOutLoud(t *testing.T) {
	gv := designerCanvas(t)
	plain := placeWidget(t, gv, "gui.Button", "plain", 10)
	selectWidgets(gv, plain)

	said, pushed := runLayoutMenuItem(t, gv, breakLayout)
	wantDeclined(t, "打破布局 on a plain button", said, pushed, "选中的控件不是布局容器")
	hasItems(t, "the scene after the declined break", gv.Scene().Children(), plain)

	box := placeWidget(t, gv, "gui.VBox", "box", 30)
	box.SetBounds(10, 30, 40, 20)
	kid, err := ged.NewFakeWidgetFromFactory("gui.Label")
	if err != nil {
		t.Fatalf("create gui.Label: %v", err)
	}
	kid.SetWidgetName("kid")
	kid.SetBounds(12, 34, 30, 6)
	kid.SetParent(box)

	selectWidgets(gv, box)
	said, pushed = runLayoutMenuItem(t, gv, breakLayout)
	wantApplied(t, "打破布局 on a layout container", said, pushed, "布局已打破")
	hasItems(t, "the scene after the break", gv.Scene().Children(), plain, kid)
}

// TestApplyVBoxLayoutWrapsSelectionUndoably is the menu half of the fix: 应用垂直
// 布局 used to detach every selected widget and push an AddCommand holding only
// the empty container, so the three widgets left the design and Ctrl+Z — which
// had recorded nothing but the container — could not bring them back. The
// handler now goes through the canvas's own undoable lay-out path.
func TestApplyVBoxLayoutWrapsSelectionUndoably(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)
	c := placeWidget(t, gv, "gui.Edit", "c", 50)
	selectWidgets(gv, a, b, c)

	applyVBoxLayout()

	box := soleContainer(t, gv, "gui.VBox")
	hasItems(t, "the VBox", box.Children(), a, b, c)
	hasItems(t, "the selection", gv.Selection().ItemList(), box)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), a, b, c)
}

// TestApplyHBoxLayoutWrapsSelectionUndoably: 应用水平布局 shared the broken
// implementation, so it needs the same guard.
func TestApplyHBoxLayoutWrapsSelectionUndoably(t *testing.T) {
	gv := designerCanvas(t)
	a := placeWidget(t, gv, "gui.Button", "a", 10)
	b := placeWidget(t, gv, "gui.Label", "b", 30)
	c := placeWidget(t, gv, "gui.Edit", "c", 50)
	selectWidgets(gv, a, b, c)

	applyHBoxLayout()

	box := soleContainer(t, gv, "gui.HBox")
	hasItems(t, "the HBox", box.Children(), a, b, c)
	hasItems(t, "the selection", gv.Selection().ItemList(), box)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), a, b, c)
}

// TestBreakLayoutLiftsChildrenUndoably: 打破布局 used to detach the container
// itself, which takes its children with it — the widgets vanished along with
// the box and nothing was pushed for Ctrl+Z to undo. The children must end up
// at the container's parent, and undo must re-nest them.
func TestBreakLayoutLiftsChildrenUndoably(t *testing.T) {
	gv := designerCanvas(t)
	box := placeWidget(t, gv, "gui.VBox", "box", 10)
	box.SetBounds(10, 10, 40, 40)

	a, err := ged.NewFakeWidgetFromFactory("gui.Button")
	if err != nil {
		t.Fatalf("create gui.Button: %v", err)
	}
	a.SetWidgetName("a")
	a.SetBounds(12, 14, 30, 8)
	a.SetParent(box)
	b, err := ged.NewFakeWidgetFromFactory("gui.Label")
	if err != nil {
		t.Fatalf("create gui.Label: %v", err)
	}
	b.SetWidgetName("b")
	b.SetBounds(12, 26, 30, 6)
	b.SetParent(box)

	selectWidgets(gv, box)

	breakLayout()

	// Both children lifted to the container's parent, the box gone.
	hasItems(t, "the scene after break", gv.Scene().Children(), a, b)

	gv.Scene().UndoStack().Undo()
	hasItems(t, "the scene after undo", gv.Scene().Children(), box)
	hasItems(t, "the restored container", box.Children(), a, b)
}
