package gui

import (
	"math"
	"testing"
)

type menuLayoutItem struct {
	Widget
	hints SizeHints
}

func (item *menuLayoutItem) SizeHints() SizeHints { return item.hints }

func newMenuLayoutItem(hints SizeHints) *menuLayoutItem {
	item := &menuLayoutItem{hints: hints}
	item.Init(item)
	return item
}

func TestMenuLayoutAndSizeHintsExcludeHiddenItems(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		menu := NewMenuBar()
		menu.SetVertical(vertical)
		one := newMenuLayoutItem(SizeHints{Width: 20, Height: 10})
		hidden := newMenuLayoutItem(SizeHints{Width: 500, Height: 500})
		two := newMenuLayoutItem(SizeHints{Width: 30, Height: 15})
		menu.AddWidget(one)
		menu.AddWidget(hidden)
		menu.AddWidget(two)
		hidden.Hide()
		hints := menu.SizeHints()
		margin := Theme().MenuBarMargin
		wantW, wantH := 50+margin.L+margin.R, 15+margin.T+margin.B
		if vertical {
			wantW, wantH = 30+margin.L+margin.R, 25+margin.T+margin.B
		}
		if hints.Width != wantW || hints.Height != wantH {
			t.Fatalf("vertical=%v includes hidden size: %+v want %v/%v", vertical, hints, wantW, wantH)
		}
		menu.SetSize(hints.Width, hints.Height)
		menu.Layout()
		if vertical && two.Y() != one.Y()+one.Height() || !vertical && two.X() != one.X()+one.Width() {
			t.Fatalf("vertical=%v hidden item leaves a layout gap", vertical)
		}
	}
}

func TestVerticalMenuExpandsAgainstItsHeightNotWidth(t *testing.T) {
	menu := NewMenuBar()
	menu.SetVertical(true)
	one := newMenuLayoutItem(SizeHints{Width: 20, Height: 10, Policy: ExpandVertical})
	two := newMenuLayoutItem(SizeHints{Width: 20, Height: 10, Policy: ExpandVertical})
	menu.AddWidget(one)
	menu.AddWidget(two)
	menu.SetSize(300, 100)
	menu.Layout()
	wantBottom := menu.Height() - Theme().MenuBarMargin.B
	if actual := two.Y() + two.Height(); math.Abs(actual-wantBottom) > 1e-9 {
		t.Fatalf("vertical menu bottom=%v, want %v; width was used as vertical free space", actual, wantBottom)
	}
}
