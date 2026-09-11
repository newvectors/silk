package gui

import "testing"

func TestDateCalendarPopupStartsHiddenAndUnattached(t *testing.T) {
	owner := NewDatePicker()
	owner.SetDate(2026, 9, 11)
	popup := newDateCalendarPopup(owner)
	if popup.IsVisible() {
		t.Fatal("calendar is visible before its native size and placement are ready")
	}
	if popup.Window() != nil || popup.NakedWidget().wt != WtInherit {
		t.Fatal("constructing a calendar created a native window")
	}
	if popup.Parent() != owner || popup.owner != owner {
		t.Fatal("calendar lost its date-picker owner")
	}
	if popup.viewYear != 2026 || popup.viewMonth != 9 || popup.hoverDay != -1 {
		t.Fatal("calendar did not initialize from the selected date")
	}
}

func TestDateCalendarSelectionClosesAndNextOpenUsesSelectedMonth(t *testing.T) {
	owner := NewDatePicker()
	owner.SetDate(2026, 9, 11)
	popup := newDateCalendarPopup(owner)
	owner.popup = popup
	// Model a visible calendar without ShowAsPopup (which creates a real
	// native window). Day selection and dismissal use the production handler.
	popup.SetSize(calCellW*calCols+8, calHeaderH+calDayRowH+calCellH*calRows+8)
	popup.NakedWidget().hidden = false
	popup.nextMonth()
	const day = 14
	cell := dayOfWeek(2026, 10, 1) + day - 1
	x := 4 + (float64(cell%calCols)+0.5)*calCellW
	y := calHeaderH + calDayRowH + (float64(cell/calCols)+0.5)*calCellH
	popup.OnLeftDown(x, y)
	if owner.Year() != 2026 || owner.Month() != 10 || owner.Day() != day {
		t.Fatal("day selection did not update the owner")
	}
	if owner.popup.IsVisible() || owner.popup.Window() != nil {
		t.Fatal("date selection did not close the calendar")
	}

	// showPopup's closed-calendar branch constructs the next popup from the
	// owner's selected date; it must not reuse stale navigation state or show
	// an unpositioned native window during construction.
	next := newDateCalendarPopup(owner)
	if next.IsVisible() || next.Window() != nil {
		t.Fatal("reopening constructs a prematurely visible calendar")
	}
	if next.viewYear != 2026 || next.viewMonth != 10 || next.Parent() != owner {
		t.Fatal("next calendar lost the selected month or owner")
	}
}
