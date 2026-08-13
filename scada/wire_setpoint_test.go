package scada

import (
	"strings"
	"testing"
	"time"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/eventlog"
	"github.com/uk0/silk/gui"
)

// eventsFrom returns every event booked for source in the last hour, queried
// across ALL kinds on purpose: filtering the query by KindWrite would make the
// row's own Kind an echo of the query rather than something the test proves.
func eventsFrom(t *testing.T, s *Services, source string) []eventlog.Event {
	t.Helper()
	now := time.Now()
	events, err := s.Events.Query(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("event query: %v", err)
	}
	var out []eventlog.Event
	for _, e := range events {
		if e.Source == source {
			out = append(out, e)
		}
	}
	return out
}

// bindSetpointScreen builds the smallest screen holding one tag-bound
// SetpointBox and wires it through the production BindScreen. Nothing seeds the
// tag: every write asserted below has to come from the widget's own commit.
func bindSetpointScreen(t *testing.T) (*Services, *gui.SetpointBox, func()) {
	t.Helper()
	s := newTestServices(t)

	root := gui.NewVBox()
	sp := gui.NewSetpointBox()
	sp.SetTagName("sp")
	sp.SetRange(0, 200)
	sp.SetValue(0)
	root.AddWidget(sp)

	stop, err := BindScreen(s, root, ScreenOptions{})
	if err != nil {
		t.Fatalf("BindScreen: %v", err)
	}
	return s, sp, stop
}

// TestBindScreenSetpointWritesTagOnCommit drives the real operator path — edit
// the field, press Enter — and asserts the write reached BOTH the tag and the
// audit log.
//
// It is also the case-order test. bindIndustrial's `case interface{
// SetValue(float64) }` matches a SetpointBox too, so moving the SetpointBox case
// below it hands back the read-only binding: SigCommit is never installed, the
// commit reaches nothing, and "sp" stays at its created zero value. The Enter is
// dispatched through the widget's own OnKeyDown rather than by calling Commit()
// directly, so the keyboard seam is part of what is proven.
func TestBindScreenSetpointWritesTagOnCommit(t *testing.T) {
	s, sp, stop := bindSetpointScreen(t)
	defer stop()

	sp.SetValue(42)                   // the operator edits the field ...
	sp.OnKeyDown(gui.KeyEnter, false) // ... and confirms it

	tag, ok := s.Tags.Get("sp")
	if !ok {
		t.Fatal("tag 'sp' was never created; the SetpointBox's TagName seam did not resolve")
	}
	if got := tag.Value().Raw; got != 42.0 {
		t.Fatalf("tag 'sp' = %#v after the commit, want 42; the commit never reached the tag", got)
	}

	rows := eventsFrom(t, s, "sp")
	if len(rows) != 1 {
		t.Fatalf("audit rows for 'sp' = %d, want exactly 1: %+v", len(rows), rows)
	}
	if rows[0].Kind != eventlog.KindWrite {
		t.Errorf("audit row kind = %q, want %q — an operator write is not a system event", rows[0].Kind, eventlog.KindWrite)
	}
	if !strings.Contains(rows[0].Message, "42") {
		t.Errorf("audit row message = %q, want the committed value in it", rows[0].Message)
	}
}

// TestBindScreenSetpointDoesNotWriteBackOnRefresh: the tag->widget refresh
// reaches the widget as a plain SetValue (gui.BindTagAnimated drives exactly
// that setter). Firing the commit callback from SetValue turns every displayed
// sample into a tag write — a loop between the two directions of the same
// binding, and a bogus KindWrite row per sample in the operator's audit log.
func TestBindScreenSetpointDoesNotWriteBackOnRefresh(t *testing.T) {
	s, sp, stop := bindSetpointScreen(t)
	defer stop()

	tag, ok := s.Tags.Get("sp")
	if !ok {
		t.Fatal("tag 'sp' was never created; the SetpointBox's TagName seam did not resolve")
	}

	// Count publications on the tag itself: Subscribe primes once, so anything
	// past that baseline is a write the display path caused.
	var published int
	cancel := tag.Subscribe(func(core.Value) { published++ })
	defer cancel()
	baseline := published

	sp.SetValue(7) // the display path a tag refresh takes

	if n := published - baseline; n != 0 {
		t.Errorf("a display-only SetValue published %d tag samples, want 0", n)
	}
	if got := tag.Value().Raw; got != nil {
		t.Errorf("tag 'sp' = %#v after a display-only SetValue, want it untouched", got)
	}
	if rows := eventsFrom(t, s, "sp"); len(rows) != 0 {
		t.Errorf("a display-only SetValue booked %d audit rows: %+v", len(rows), rows)
	}
}

// TestBindScreenSetpointStopDetachesCommit: the stop func BindScreen returns
// must detach the write side as well as the read side. Dropping the
// SigCommit(nil) leaves a closed screen's field still writing tags — and still
// booking audit rows — from a Services the host has torn down.
func TestBindScreenSetpointStopDetachesCommit(t *testing.T) {
	s, sp, stop := bindSetpointScreen(t)

	sp.SetValue(42)
	sp.OnKeyDown(gui.KeyEnter, false)

	tag, ok := s.Tags.Get("sp")
	if !ok {
		t.Fatal("tag 'sp' was never created; the SetpointBox's TagName seam did not resolve")
	}
	if tag.Value().Raw != 42.0 {
		t.Fatalf("tag 'sp' = %#v before stop, want 42; the wiring never worked", tag.Value().Raw)
	}

	stop()

	sp.SetValue(99)
	sp.OnKeyDown(gui.KeyEnter, false)

	if got := tag.Value().Raw; got != 42.0 {
		t.Errorf("tag 'sp' = %#v after a commit on a stopped screen, want it still 42", got)
	}
	if rows := eventsFrom(t, s, "sp"); len(rows) != 1 {
		t.Errorf("audit rows for 'sp' = %d after a commit on a stopped screen, want the 1 from before stop: %+v", len(rows), rows)
	}
}
