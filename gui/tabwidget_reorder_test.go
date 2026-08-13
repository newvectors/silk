package gui

import "testing"

// newTabWidgetABC builds a laid-out three-page TabWidget whose pages name
// themselves: tab "A" owns the label reading "pageA". Both halves of every
// tab/page pair are readable in an assertion, which is the whole point here --
// the bug is a pairing bug, not an ordering bug.
func newTabWidgetABC() *TabWidget {
	tw := NewTabWidget()
	tw.AddTab(NewLabel("pageA"), "A", nil)
	tw.AddTab(NewLabel("pageB"), "B", nil)
	tw.AddTab(NewLabel("pageC"), "C", nil)
	hints := tw.SizeHints()
	tw.SetBounds(0, 0, hints.Width, hints.Height)
	tw.Layout()
	return tw
}

// newTabWidgetABCD builds the four-page shape the IDE editor strip actually
// runs in: buildEditorTabs seeds three tabs and grows by one per file opened.
// Four is the first count where a tab can move and still leave a tab standing
// outside the span that moved -- that bystander keeps its own page or the
// reorder is wrong, and no three-tab end-to-end move produces one.
func newTabWidgetABCD() *TabWidget {
	tw := NewTabWidget()
	for _, title := range []string{"A", "B", "C", "D"} {
		tw.AddTab(NewLabel("page"+title), title, nil)
	}
	hints := tw.SizeHints()
	tw.SetBounds(0, 0, hints.Width, hints.Height)
	tw.Layout()
	return tw
}

// tabTitleAt reads the title the strip paints at index i.
func tabTitleAt(tb *TabBar, i int) string {
	ref, _ := tb.Data(i).(*tabPageRef)
	if ref == nil {
		return "<nil>"
	}
	return ref.title
}

// pageTextAt reads the page sitting in stack slot i.
func pageTextAt(tw *TabWidget, i int) string {
	l, _ := tw.Stack().Page(i).(*Label)
	if l == nil {
		return "<nil>"
	}
	return l.Text()
}

// currentPageText reads the page the stack calls current.
func currentPageText(tw *TabWidget) string {
	l, _ := tw.Stack().CurrentPage().(*Label)
	if l == nil {
		return "<nil>"
	}
	return l.Text()
}

// visiblePageTexts lists the pages actually on screen. A stack ever shows one
// page, so this is what the user sees -- CurrentPage() only says what the stack
// believes it is showing.
func visiblePageTexts(tw *TabWidget) (texts []string) {
	for i := 0; i < tw.Stack().Count(); i++ {
		if p := tw.Stack().Page(i); p != nil && p.IsVisible() {
			texts = append(texts, pageTextAt(tw, i))
		}
	}
	return
}

// dragTab drags tab `from` to the drop point dropX through the production path:
// the press arms downTab exactly as a real click does, and the drop carries the
// context shape Widget.DoDragDrop builds for a strip dragging onto itself.
func dragTab(t *testing.T, tb *TabBar, from int, dropX float64) {
	t.Helper()
	_, _, _, h := tb.Bounds()
	my := h * 0.5

	tb.OnLeftDown(tabCellStart(tb, from)+tb.tabs[from].width*0.5, my)
	if tb.downTab != from {
		t.Fatalf("press on tab %d armed downTab = %d, want %d", from, tb.downTab, from)
	}
	tb.OnDrop(dropX, my, &dndContext{pa: DndMove, from: tb.Self()})
}

// dropPointAt returns an x whose DropIndex is want, and fails the test if the
// strip disagrees -- the drop has to be aimed where the user aimed it.
func dropPointAt(t *testing.T, tb *TabBar, want int) float64 {
	t.Helper()
	var x float64
	if want >= tb.Count() {
		last := tb.Count() - 1
		x = tabCellStart(tb, last) + tb.tabs[last].width - 0.5
	} else {
		x = tabCellStart(tb, want) + 0.5
	}
	_, _, _, h := tb.Bounds()
	if got := tb.DropIndex(x, h*0.5); got != want {
		t.Fatalf("DropIndex(%.2f) = %d, want %d", x, got, want)
	}
	return x
}

// assertPaired checks that every strip slot and the page slot beside it name the
// same logical tab: tab "B" must sit next to "pageB".
func assertPaired(t *testing.T, tw *TabWidget) {
	t.Helper()
	for i := 0; i < tw.TabBar().Count(); i++ {
		title := tabTitleAt(tw.TabBar(), i)
		if got, want := pageTextAt(tw, i), "page"+title; got != want {
			t.Errorf("slot %d: tab %q is paired with page %q, want %q", i, title, got, want)
		}
	}
}

// assertStripOrder checks the titles the strip paints, left to right. Pairing
// alone is not enough on its own: a drop that moved nothing keeps every tab
// beside its own page, so the landing order has to be pinned too.
func assertStripOrder(t *testing.T, tb *TabBar, want ...string) {
	t.Helper()
	if got := tb.Count(); got != len(want) {
		t.Fatalf("strip holds %d tabs, want %d %v", got, len(want), want)
	}
	for i, w := range want {
		if got := tabTitleAt(tb, i); got != w {
			t.Errorf("strip slot %d = %q, want %q", i, got, w)
		}
	}
}

// assertEveryTabShowsItsPage switches to each tab in turn and checks the page
// that ends up on screen is the one that tab owns -- what the user meets after
// a reorder loses track of a page.
func assertEveryTabShowsItsPage(t *testing.T, tw *TabWidget) {
	t.Helper()
	tb := tw.TabBar()
	for i := 0; i < tb.Count(); i++ {
		tw.SetCurrentIndex(i)
		title := tabTitleAt(tb, i)
		if got := visiblePageTexts(tw); len(got) != 1 || got[0] != "page"+title {
			t.Errorf("switching to slot %d (tab %q) leaves %v on screen, want [page%s]",
				i, title, got, title)
		}
	}
}

// TestTabWidgetDragReorderKeepsPagesPaired drags the first tab past the last
// one. TabBar.OnDrop reorders its own strip and told nobody, so the TabWidget's
// refs and the stack's pages kept the old order: the strip read [B C A] while
// the pages stayed [pageA pageB pageC]. Every tab then addressed somebody
// else's page -- the drop itself left tab A active with pageC on screen.
func TestTabWidgetDragReorderKeepsPagesPaired(t *testing.T) {
	tw := newTabWidgetABC()
	tb := tw.TabBar()

	dragTab(t, tb, 0, dropPointAt(t, tb, tb.Count()))

	if got, want := tb.Count(), 3; got != want {
		t.Fatalf("tab count after drop = %d, want %d", got, want)
	}
	if got, want := tw.Stack().Count(), 3; got != want {
		t.Fatalf("page count after drop = %d, want %d", got, want)
	}

	// (a) strip [B C A] must pair with pages [pageB pageC pageA].
	assertPaired(t, tw)
	if got, want := tabTitleAt(tb, 2), "A"; got != want {
		t.Fatalf("dropped tab landed at strip slot 2 as %q, want %q", got, want)
	}

	// (c) The dropped tab is the active one, and its own page is what shows.
	if got, want := tb.ActiveTab(), 2; got != want {
		t.Errorf("active tab after drop = %d (%q), want %d (\"A\")",
			got, tabTitleAt(tb, got), want)
	}
	if got, want := visiblePageTexts(tw), "pageA"; len(got) != 1 || got[0] != want {
		t.Errorf("visible page(s) after dropping tab A = %v, want [%s]", got, want)
	}
	if got, want := currentPageText(tw), "pageA"; got != want {
		t.Errorf("current page after dropping tab A = %q, want %q", got, want)
	}

	// (b) And every later switch lands on the page of the tab switched to.
	for i := 0; i < tb.Count(); i++ {
		tw.SetCurrentIndex(i)
		title := tabTitleAt(tb, i)
		if got := currentPageText(tw); got != "page"+title {
			t.Errorf("SetCurrentIndex(%d) on tab %q shows %q, want page%s",
				i, title, got, title)
		}
		if got := visiblePageTexts(tw); len(got) != 1 || got[0] != "page"+title {
			t.Errorf("SetCurrentIndex(%d) on tab %q leaves %v on screen, want [page%s]",
				i, title, got, title)
		}
	}
}

// TestTabWidgetDragReorderBackwards drags the last tab to the front, the other
// direction of the same move: the pages have to slide right while the tab slides
// left.
func TestTabWidgetDragReorderBackwards(t *testing.T) {
	tw := newTabWidgetABC()
	tb := tw.TabBar()

	dragTab(t, tb, 2, dropPointAt(t, tb, 0))

	assertPaired(t, tw)
	if got, want := tabTitleAt(tb, 0), "C"; got != want {
		t.Fatalf("dropped tab landed at strip slot 0 as %q, want %q", got, want)
	}
	if got, want := tb.ActiveTab(), 0; got != want {
		t.Errorf("active tab after drop = %d (%q), want %d (\"C\")",
			got, tabTitleAt(tb, got), want)
	}
	if got, want := visiblePageTexts(tw), "pageC"; len(got) != 1 || got[0] != want {
		t.Errorf("visible page(s) after dropping tab C = %v, want [%s]", got, want)
	}
}

// TestTabWidgetDragReorderMovesOnlyTheSpan drags a tab across a four-tab strip
// and leaves a tab standing past the move. A reorder renumbers only the slots
// between the tab's old and new position; everything outside that span keeps
// the page it already had. The two tests above move an end tab to the other
// end, where the span is the whole strip and there is no outside to get wrong.
func TestTabWidgetDragReorderMovesOnlyTheSpan(t *testing.T) {
	// Forward: A moves one slot right, [A B C D] -> [B A C D]. C and D sit
	// past the drop and are none of the move's business.
	t.Run("forward", func(t *testing.T) {
		tw := newTabWidgetABCD()
		tb := tw.TabBar()

		dragTab(t, tb, 0, dropPointAt(t, tb, 2))

		assertStripOrder(t, tb, "B", "A", "C", "D")
		assertPaired(t, tw)
		if got, want := currentPageText(tw), "pageA"; got != want {
			t.Errorf("current page after dropping tab A = %q, want %q", got, want)
		}
		assertEveryTabShowsItsPage(t, tw)
	})

	// Backward: C is dragged out of the middle to the front,
	// [A B C D] -> [C A B D]. D sits past the tab that left and must not move.
	t.Run("backward", func(t *testing.T) {
		tw := newTabWidgetABCD()
		tb := tw.TabBar()

		dragTab(t, tb, 2, dropPointAt(t, tb, 0))

		assertStripOrder(t, tb, "C", "A", "B", "D")
		assertPaired(t, tw)
		if got, want := currentPageText(tw), "pageC"; got != want {
			t.Errorf("current page after dropping tab C = %q, want %q", got, want)
		}
		assertEveryTabShowsItsPage(t, tw)
	})
}

// TestTabWidgetDragReorderBackwardsOneSlot nudges the middle tab of the
// three-tab strip one slot left. TestTabWidgetDragReorderBackwards drags the
// last tab, and a last tab has nothing behind it to leave behind; B does.
func TestTabWidgetDragReorderBackwardsOneSlot(t *testing.T) {
	tw := newTabWidgetABC()
	tb := tw.TabBar()

	dragTab(t, tb, 1, dropPointAt(t, tb, 0))

	assertStripOrder(t, tb, "B", "A", "C")
	assertPaired(t, tw)
	assertEveryTabShowsItsPage(t, tw)
}

// TestTabBarDragReorderWithoutMoveCallback is the Dock shape: a bare strip whose
// tabs carry their own data and which registers no move callback. Reordering it
// must keep working exactly as before -- the notification is for containers that
// need it, not a requirement of the strip.
func TestTabBarDragReorderWithoutMoveCallback(t *testing.T) {
	tb := NewTabBar()
	for _, title := range []string{"A", "B", "C"} {
		tb.AddTab(&tabPageRef{title: title}, false)
	}
	hints := tb.SizeHints()
	tb.SetBounds(0, 0, hints.Width, hints.Height)
	tb.Layout()

	dragTab(t, tb, 0, dropPointAt(t, tb, tb.Count()))

	want := []string{"B", "C", "A"}
	for i, w := range want {
		if got := tabTitleAt(tb, i); got != w {
			t.Errorf("tab %d = %q, want %q", i, got, w)
		}
	}
}
