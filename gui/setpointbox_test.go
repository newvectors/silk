package gui

import "testing"

// TestSetpointBoxClampsToInheritedRange: the range clamp is NumberInput's
// (numberinput.go SetValue), and the setpoint field must keep it — a typed 150
// on a 0..100 loop has to reach the tag as 100, not as an out-of-range setpoint
// the PLC then refuses or, worse, accepts.
func TestSetpointBoxClampsToInheritedRange(t *testing.T) {
	sp := NewSetpointBox()
	sp.SetRange(0, 100)

	sp.SetValue(150)
	if got := sp.Value(); got != 100 {
		t.Errorf("SetValue(150) on a 0..100 setpoint = %v, want the clamped 100", got)
	}
}

// TestSetpointBoxCommitsOnEnterAndBlur: the commit callback is the only way a
// value leaves the widget, and it must carry the CLAMPED value. It must not
// fire on a plain SetValue — that is the path a tag refresh takes, and
// committing from there writes every displayed sample straight back to the tag.
func TestSetpointBoxCommitsOnEnterAndBlur(t *testing.T) {
	sp := NewSetpointBox()
	sp.SetRange(0, 100)

	var got []float64
	sp.SigCommit(func(v float64) { got = append(got, v) })

	sp.SetValue(150) // display path: clamps to 100, commits nothing
	if len(got) != 0 {
		t.Fatalf("SetValue committed %v; SetValue must never write back", got)
	}

	sp.OnKeyDown(KeyEnter, false)
	if len(got) != 1 || got[0] != 100 {
		t.Fatalf("commits after Enter = %v, want exactly [100]", got)
	}

	sp.OnFocusOut()
	if len(got) != 2 || got[1] != 100 {
		t.Fatalf("commits after Enter+blur = %v, want [100 100]", got)
	}
}

// TestSetpointBoxWithoutCommitCallback: a setpoint dropped in the designer and
// never bound has no callback installed, so Enter and blur must be no-ops
// rather than a nil-func panic that takes the whole screen down.
func TestSetpointBoxWithoutCommitCallback(t *testing.T) {
	sp := NewSetpointBox()
	sp.SetRange(0, 100)
	sp.SetValue(30)

	sp.OnKeyDown(KeyEnter, false)
	sp.OnFocusOut()
	sp.Commit()

	if got := sp.Value(); got != 30 {
		t.Errorf("Value() = %v after an unbound Enter/blur, want 30", got)
	}
}
