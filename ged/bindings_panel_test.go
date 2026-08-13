package ged

import (
	"testing"

	"github.com/uk0/silk/graph"
	"github.com/uk0/silk/gui"
)

// bindingsFixture is the design every case below reads. Its shape is chosen to
// distinguish, not merely to have rows:
//
//   - btnDeep sits inside a container, so a flat scene.Children() walk misses it;
//   - srchIt is a SearchBox, the one fixture widget with two events and only one
//     of them bound, so "bound rows first" is told apart from "the widgetEvents
//     table's own order";
//   - edName is bound to nothing, so the unbound half of a row is exercised.
type bindingsFixture struct {
	scene   *GedScene
	btnOK   *FakeWidget
	edName  *FakeWidget
	box     *FakeWidget
	btnDeep *FakeWidget
	srch    *FakeWidget
}

func newBindingsFixture(t *testing.T) *bindingsFixture {
	t.Helper()
	view := NewGedView()
	scene := view.GedScene()

	fx := &bindingsFixture{scene: scene}
	fx.btnOK = sceneWidget(t, scene, "gui.Button", "btnOK", 10, 10, 40, 8)
	fx.btnOK.SetEventHandlerChecked("OnClick", "onOK")
	fx.edName = sceneWidget(t, scene, "gui.Edit", "edName", 10, 30, 40, 8)
	fx.box = sceneWidget(t, scene, "gui.VBox", "boxA", 10, 50, 60, 40)
	fx.btnDeep = nestChild(t, fx.box, "btnDeep")
	fx.btnDeep.SetEventHandlerChecked("OnClick", "onDeep")
	fx.srch = sceneWidget(t, scene, "gui.SearchBox", "srchIt", 10, 100, 40, 8)
	fx.srch.SetEventHandlerChecked("OnTextChanged", "onType")
	return fx
}

// boundBindingsPanel returns a panel already listing fx.
func boundBindingsPanel(fx *bindingsFixture) *BindingsPanel {
	p := NewBindingsPanel()
	p.SetScene(fx.scene)
	return p
}

// bindingsRowY is the y coordinate at the middle of row r.
func bindingsRowY(p *BindingsPanel, r int) float64 {
	return p.contentTop() + float64(r)*p.rowHeight + p.rowHeight*0.5
}

// rowIndexOf returns the index of the row for (widget, event), or -1.
func rowIndexOf(p *BindingsPanel, widget, event string) int {
	for i, row := range p.Rows() {
		if row.Widget == widget && row.Event == event {
			return i
		}
	}
	return -1
}

// TestBindingsPanelRowsCarryTheWholeBinding pins the payload, not the labels:
// the handler name, the factory it belongs to, and the two flags are what make
// this panel worth opening — a row that says "btnOK OnClick" and does not carry
// "onOK" answers none of the questions the panel exists for. The nested row is
// the one that fails a flat scene.Children() walk.
func TestBindingsPanelRowsCarryTheWholeBinding(t *testing.T) {
	fx := newBindingsFixture(t)
	panel := boundBindingsPanel(fx)
	rows := panel.Rows()

	want := BindingRow{
		Widget: "btnOK", Factory: "gui.Button", Event: "OnClick",
		Handler: "onOK", Bound: true, Known: true,
	}
	if !hasBindingRow(rows, want) {
		t.Errorf("rows = %+v\nwant one exactly equal to %+v", rows, want)
	}

	// The Edit is wired to nothing, and that is the row the panel exists to
	// show: its event is listed, its handler is empty and Bound says so.
	unbound := BindingRow{
		Widget: "edName", Factory: "gui.Edit", Event: "OnChanged",
		Handler: "", Bound: false, Known: true,
	}
	if !hasBindingRow(rows, unbound) {
		t.Errorf("rows = %+v\nwant one exactly equal to %+v", rows, unbound)
	}

	deep := BindingRow{
		Widget: "btnDeep", Factory: "gui.Button", Event: "OnClick",
		Handler: "onDeep", Bound: true, Known: true,
	}
	if !hasBindingRow(rows, deep) {
		t.Errorf("the widget nested inside boxA is missing from %+v;\n"+
			"the walk must descend into containers the way GenerateCode does", rows)
	}
}

func hasBindingRow(rows []BindingRow, want BindingRow) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

// TestBindingsPanelListsInDocumentOrder pins the whole sequence. Two orderings
// this catches and a "contains" assertion does not: a walk that lists a
// container's children after all its siblings (btnDeep last), and a widget
// listed straight off the widgetEvents table (srchIt's unbound OnSearch ahead
// of its bound OnTextChanged).
func TestBindingsPanelListsInDocumentOrder(t *testing.T) {
	fx := newBindingsFixture(t)
	panel := boundBindingsPanel(fx)

	want := [][2]string{
		{"btnOK", "OnClick"},
		{"edName", "OnChanged"},
		{"btnDeep", "OnClick"},
		{"srchIt", "OnTextChanged"},
		{"srchIt", "OnSearch"},
	}
	got := make([][2]string, 0, len(want))
	for _, row := range panel.Rows() {
		got = append(got, [2]string{row.Widget, row.Event})
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %v, want %v (full list %v)", i, got[i], want[i], got)
		}
	}
}

// TestBindingsPanelFlagsUnknownEvent: a handler recorded against an event
// codegen has no entry for generates nothing at all. The property sheet cannot
// produce one — it offers only AvailableEvents — but a hand-edited .silkui and
// a widget morphed to another type both can, and the row is the only place that
// says so before the build does.
func TestBindingsPanelFlagsUnknownEvent(t *testing.T) {
	fx := newBindingsFixture(t)
	// Bypass SetEventHandlerChecked: it validates the handler, not the event.
	fx.btnOK.SetEventHandler("OnBogus", "x")

	panel := boundBindingsPanel(fx)
	idx := rowIndexOf(panel, "btnOK", "OnBogus")
	if idx < 0 {
		t.Fatalf("the recorded OnBogus binding is not listed at all: %+v", panel.Rows())
	}
	if row := panel.Rows()[idx]; row.Known || !row.Bound || row.Handler != "x" {
		t.Errorf("OnBogus row = %+v, want Bound=true Handler=\"x\" Known=false", row)
	}
	// The events codegen does know must not be tarred with the same brush.
	if row := panel.Rows()[rowIndexOf(panel, "btnOK", "OnClick")]; !row.Known {
		t.Errorf("OnClick row = %+v, want Known=true", row)
	}
}

// TestBindingsPanelFollowsDesignChanges drives the real call site: a command
// pushed onto the scene's stack, no Rebuild() from the test. Dropping the
// AddDesignChangedCallback subscription in SetScene leaves the panel listing
// the design as it was when the tab was opened — the stale-panel defect the
// 生成代码 view was reported with.
func TestBindingsPanelFollowsDesignChanges(t *testing.T) {
	fx := newBindingsFixture(t)
	panel := boundBindingsPanel(fx)
	before := len(panel.Rows())

	chk, err := NewFakeWidgetFromFactory("gui.CheckBox")
	if err != nil {
		t.Fatalf("create gui.CheckBox: %v", err)
	}
	chk.SetWidgetName("chkAgree")
	chk.SetBounds(10, 120, 40, 8)
	cmd := graph.NewAddCommand()
	cmd.AddItem(chk, fx.scene)
	fx.scene.PushCommand(cmd)

	rows := panel.Rows()
	if len(rows) != before+1 {
		t.Fatalf("after adding a CheckBox the panel lists %d row(s), want %d — it never heard about the command",
			len(rows), before+1)
	}
	last := rows[len(rows)-1]
	if last.Widget != "chkAgree" || last.Event != "OnToggled" || last.Bound {
		t.Errorf("the new row = %+v, want the CheckBox's own unbound OnToggled", last)
	}
}

// TestBindingsPanelDoesNotStealTheDesignSlot: SigDesignChanged holds exactly one
// callback and the code panel owns it. A panel that refreshes itself by
// assigning that slot silently unplugs whoever registered first — here, a
// listener registered BEFORE the panel was bound.
func TestBindingsPanelDoesNotStealTheDesignSlot(t *testing.T) {
	fx := newBindingsFixture(t)
	fired := 0
	fx.scene.SigDesignChanged(func() { fired++ })

	panel := boundBindingsPanel(fx)
	before := len(panel.Rows())

	lbl, err := NewFakeWidgetFromFactory("gui.Link")
	if err != nil {
		t.Fatalf("create gui.Link: %v", err)
	}
	lbl.SetWidgetName("lnkHelp")
	lbl.SetBounds(10, 120, 40, 8)
	cmd := graph.NewAddCommand()
	cmd.AddItem(lbl, fx.scene)
	fx.scene.PushCommand(cmd)

	if fired != 1 {
		t.Errorf("the listener registered before the panel fired %d time(s), want 1 — the panel took its slot", fired)
	}
	if len(panel.Rows()) != before+1 {
		t.Errorf("the panel lists %d row(s), want %d — it did not follow the command", len(panel.Rows()), before+1)
	}
}

// bindingsRebuildCounter is a BindingsPanel that counts what a design change
// costs it. Rebuild ends in Self().Update(), and in the case below nothing else
// asks the panel to repaint, so the count is the number of rebuilds one pushed
// command produced.
type bindingsRebuildCounter struct {
	BindingsPanel
	updates int
}

func (this *bindingsRebuildCounter) Update() { this.updates++ }

func newBindingsRebuildCounter() *bindingsRebuildCounter {
	p := new(bindingsRebuildCounter)
	p.Init(p)
	return p
}

// pushAddedWidget adds a widget to scene through a command on its stack — the
// path a drop, a paste and their undo/redo all share, and the one that reports
// the design changed.
func pushAddedWidget(t *testing.T, scene *GedScene, factory, name string) *FakeWidget {
	t.Helper()
	fake, err := NewFakeWidgetFromFactory(factory)
	if err != nil {
		t.Fatalf("create %s: %v", factory, err)
	}
	fake.SetWidgetName(name)
	fake.SetBounds(10, 120, 40, 8)
	cmd := graph.NewAddCommand()
	cmd.AddItem(fake, scene)
	scene.PushCommand(cmd)
	return fake
}

// TestBindingsPanelListensOnlyToTheDesignItShows counts the subscriptions from
// the far end. The panel is a singleton the tab change re-aims, so a
// subscription taken per SetScene accumulates: two visits to a design leave
// every edit there rebuilding the panel twice, three visits three times, for as
// long as the session lasts — and a design the user has switched away from goes
// on driving a panel that shows another document.
func TestBindingsPanelListensOnlyToTheDesignItShows(t *testing.T) {
	first := newBindingsFixture(t)
	second := newBindingsFixture(t)
	panel := newBindingsRebuildCounter()

	panel.SetScene(first.scene)
	panel.SetScene(second.scene)

	// An edit on the design the panel was re-aimed away from — an undo on a
	// background tab, a property edit from the object tree — is no longer its
	// business.
	panel.updates = 0
	pushAddedWidget(t, first.scene, "gui.CheckBox", "chkBackground")
	if panel.updates != 0 {
		t.Errorf("a command on the design the panel was re-aimed away from rebuilt it %d time(s), want 0 — SetScene left the old subscription live", panel.updates)
	}

	// Re-aimed back, one command must cost exactly one rebuild, not one per
	// visit the panel has paid this design.
	panel.SetScene(first.scene)
	panel.updates = 0
	pushAddedWidget(t, first.scene, "gui.CheckBox", "chkAgree")
	if panel.updates != 1 {
		t.Errorf("back on the first design one command rebuilt the panel %d time(s), want 1 — SetScene subscribes again without dropping the subscription it already held", panel.updates)
	}
	if rowIndexOf(&panel.BindingsPanel, "chkAgree", "OnToggled") < 0 {
		t.Errorf("the widget added after re-aiming back is not listed: %+v", panel.Rows())
	}
}

// TestBindingsPanelPressReportsTheWidget: pressing a row hands the caller the
// widget that row describes — the identical item, not one found by name. Both
// presses are rows a naive implementation gets wrong: a nested widget's row,
// and an unbound row (a rows/items pair filled only on the bound branch is off
// by one from there on).
func TestBindingsPanelPressReportsTheWidget(t *testing.T) {
	fx := newBindingsFixture(t)
	panel := boundBindingsPanel(fx)

	var got graph.IItem
	panel.SigSelect(func(it graph.IItem) { got = it })

	deep := rowIndexOf(panel, "btnDeep", "OnClick")
	if deep < 0 {
		t.Fatalf("btnDeep has no row: %+v", panel.Rows())
	}
	panel.OnLeftDown(10, bindingsRowY(panel, deep))
	if got != graph.IItem(fx.btnDeep) {
		t.Errorf("pressing btnDeep's row reported %v, want the nested widget itself", got)
	}
	if panel.SelectedRow() != deep {
		t.Errorf("selected row = %d, want %d", panel.SelectedRow(), deep)
	}

	unbound := rowIndexOf(panel, "srchIt", "OnSearch")
	panel.OnLeftDown(10, bindingsRowY(panel, unbound))
	if got != graph.IItem(fx.srch) {
		t.Errorf("pressing the unbound OnSearch row reported %v, want srchIt", got)
	}
}

// TestBindingsPanelIsAToolViewNotAWidget: it reaches the 视图 menu through the
// tool-view registry, and must never reach the palette — dragging an IDE panel
// onto a form generates core.New("ged.BindingsPanel").(gui.IWidget) into a
// program that does not link ged, which asserts on nil and panics at startup.
func TestBindingsPanelIsAToolViewNotAWidget(t *testing.T) {
	def, ok := gui.GetToolViewDef("ged.BindingsPanel")
	if !ok {
		t.Fatal("ged.BindingsPanel is not a registered tool view; the 视图 menu is built from that registry")
	}
	if def.Name != "信号/槽" {
		t.Errorf("tool view name = %q, want 信号/槽", def.Name)
	}
	if isPlaceable("ged.BindingsPanel") {
		t.Error("the bindings panel is placeable; an IDE panel must have no factoryMap entry")
	}
	for _, name := range paletteNames() {
		if name == "ged.BindingsPanel" {
			t.Error("the bindings panel is offered by the widget palette")
		}
	}
}
