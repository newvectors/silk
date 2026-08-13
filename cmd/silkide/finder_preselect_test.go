package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/uk0/silk/gui"
)

// Quick Open, the command palette and Go to Symbol all rebuild their list on
// every keystroke, and ListWidget.Clear resets the selection to -1. Nothing was
// putting it back, so the dialogs opened with no row highlighted and Enter on
// the list — which runs list.ActiveIndex() — did nothing at all until the user
// clicked a row. fillFinderList is the one seam all three now fill through.

// TestFillFinderListSelectsTheTopRow: after a fill the first row is the
// selected one, whatever was selected before. The identity check is the point —
// a list refilled while row 2 was selected has to come back on the row the new
// ranking put on top, not on whatever now happens to sit at the old index.
func TestFillFinderListSelectsTheTopRow(t *testing.T) {
	list := gui.NewListWidget()

	fillFinderList(list, []gui.ListItem{
		{Text: "main.go"}, {Text: "gui/button.go"}, {Text: "gui/list-widget.go"},
	})
	if got := list.ActiveIndex(); got != 0 {
		t.Fatalf("after fill: ActiveIndex = %d, want 0 (nothing highlighted, Enter dead)", got)
	}
	if got, want := list.Item(list.ActiveIndex()).Text, "main.go"; got != want {
		t.Errorf("selected row = %q, want the first item passed in, %q", got, want)
	}

	// The user arrows down to the third row, then types another character: the
	// refill has to move the selection back to the top of the new ranking.
	list.SetActiveIndex(2)
	fillFinderList(list, []gui.ListItem{
		{Text: "core/uuid.go"}, {Text: "core/log.go"}, {Text: "core/factory.go"},
	})
	if got := list.ActiveIndex(); got != 0 {
		t.Fatalf("after refill: ActiveIndex = %d, want 0", got)
	}
	if got, want := list.Item(list.ActiveIndex()).Text, "core/uuid.go"; got != want {
		t.Errorf("selected row after refill = %q, want the first item of the new results, %q", got, want)
	}
}

// TestFillFinderListWithNoMatchesSelectsNothing: a query that matches nothing
// leaves an empty list, and "nothing selected" is the only honest answer — row
// 0 of an empty list is a row Item() panics on.
func TestFillFinderListWithNoMatchesSelectsNothing(t *testing.T) {
	list := gui.NewListWidget()
	fillFinderList(list, []gui.ListItem{{Text: "main.go"}})
	fillFinderList(list, nil)

	if got := list.ActiveIndex(); got != -1 {
		t.Errorf("after filling with 0 items: ActiveIndex = %d, want -1", got)
	}
}

// finderDialogs maps each finder that rebuilds a ListWidget to the file it
// lives in.
var finderDialogs = map[string]string{
	"showCommandPalette": "palette.go",
	"showFileFinder":     "file_finder.go",
	"showSymbolFinder":   "symbol_finder.go",
}

// TestEveryFinderFillsThroughFillFinderList: the test above proves the seam
// works, not that the dialogs use it — converting two of the three would leave
// the third opening with a dead Enter and the package still green. So this
// reads the three functions out of the syntax tree (they open modal dialogs and
// cannot be called from a test) and pins that each one fills through
// fillFinderList and none of them clears the list behind its back.
func TestEveryFinderFillsThroughFillFinderList(t *testing.T) {
	for name, file := range finderDialogs {
		fn := findFuncDecl(t, file, name)
		if !callsFunc(fn.Body, "fillFinderList") {
			t.Errorf("%s (%s) never calls fillFinderList; its list opens with no row selected and Enter on it does nothing", name, file)
		}
		if recv := clearedReceiver(fn.Body); recv != "" {
			t.Errorf("%s (%s) calls %s.Clear() directly; Clear drops the selection to -1, which is exactly what fillFinderList exists to put back", name, file, recv)
		}
	}
}

// findFuncDecl parses file (relative to the package dir, the way the test runs)
// and returns the named top-level function.
func findFuncDecl(t *testing.T, file, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", file, err)
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	t.Fatalf("%s has no func %s; update finderDialogs to match", file, name)
	return nil
}

// callsFunc reports whether body calls the plain (non-method) function name,
// including from inside a closure — showSymbolFinder fills its list from a
// gui.Post callback nested in a goroutine.
func callsFunc(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// clearedReceiver returns the name of the first variable body calls Clear() on,
// or "" when it calls none.
func clearedReceiver(body *ast.BlockStmt) (recv string) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return recv == ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Clear" {
			return recv == ""
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			recv = id.Name
		} else {
			recv = "it"
		}
		return false
	})
	return
}
