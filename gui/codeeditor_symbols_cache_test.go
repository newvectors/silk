package gui

import (
	"reflect"
	"strings"
	"testing"
)

func TestCodeSymbolCacheParsesOncePerRevisionAndLanguage(t *testing.T) {
	var cache codeSymbolCache
	calls := 0
	parse := func(lines []string) []CodeSymbol {
		calls++
		return parseGoCodeSymbols(lines)
	}
	lines := []string{"package p", "func TestAlpha() {}"}
	key := codeSymbolCacheKey{revision: 1, language: codeSymbolLanguageGo}
	for i := 0; i < 100; i++ {
		cache.load(key, lines, parse)
		if symbol := cache.functionAt(1); symbol == nil || symbol.Name != "TestAlpha" {
			t.Fatal("cached breadcrumb did not resolve the function")
		}
		if cache.testLines[1] != "TestAlpha" {
			t.Fatal("cached test projection did not resolve the function")
		}
	}
	if calls != 1 {
		t.Fatalf("unchanged document parsed %d times, want 1", calls)
	}
	key.revision++
	cache.load(key, lines, parse)
	if calls != 2 {
		t.Fatal("new revision did not reparse")
	}
	key.language = "another-dialect"
	cache.load(key, lines, parse)
	cache.load(key, lines, parse)
	if calls != 3 {
		t.Fatalf("language-key change parsed %d total times, want 3", calls)
	}
	// Nil results must also hit the cache, including a zero-value revision.
	var empty codeSymbolCache
	before := calls
	for i := 0; i < 100; i++ {
		empty.load(codeSymbolCacheKey{}, []string{"// no declarations"}, parse)
	}
	if calls != before+1 || !empty.valid || len(empty.symbols) != 0 {
		t.Fatal("empty parse result was not cached")
	}
}

func TestCodeEditorSymbolsCacheCopySemanticsAndDrawQueries(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("package p\ntype Server struct{}\nfunc (s *Server) Run() {}\nfunc TestAlpha() {}\n")
	e.filePath = "cache_test.go" // avoid SetFilePath's unrelated git subprocess
	e.cursorLine = 3
	got := e.ParseSymbols()
	want := parseGoCodeSymbols(e.lines)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cached symbols = %#v, want %#v", got, want)
	}
	storage := &e.symbolCache.symbols[0]
	got[0].Name = "corrupted"
	got[1].Kind = SymConst
	got[2].Line = 999
	if !reflect.DeepEqual(e.ParseSymbols(), want) {
		t.Fatal("public ParseSymbols snapshot exposed cache storage")
	}
	if symbol := e.currentFunction(); symbol == nil || symbol.Name != "TestAlpha" {
		t.Fatal("breadcrumb was corrupted by public snapshot mutation")
	}
	if e.testFuncLines()[3] != "TestAlpha" {
		t.Fatal("gutter was corrupted by public snapshot mutation")
	}
	for i := 0; i < 100; i++ {
		e.cursorLine = i % 5
		e.currentFunction()
		e.testFuncLines()
	}
	if &e.symbolCache.symbols[0] != storage {
		t.Fatal("drawing/caret changes replaced parsed symbol storage")
	}
}

func TestCodeEditorSymbolsCacheInvalidatesOnEditUndoAndFilename(t *testing.T) {
	e := NewCodeEditor()
	const initial = "package p\nfunc TestAlpha() {}\n"
	const changed = "package p\nfunc TestOmega() {}\n"
	e.SetText(initial)
	e.filePath = "cache_test.go"
	e.cursorLine = 1
	if e.testFuncLines()[1] != "TestAlpha" {
		t.Fatal("fixture test marker is missing")
	}
	e.ReplaceAllText(changed)
	if e.testFuncLines()[1] != "TestOmega" || e.currentFunction().Name != "TestOmega" {
		t.Fatal("same-length edit did not invalidate all symbol projections")
	}
	e.Undo()
	if e.testFuncLines()[1] != "TestAlpha" {
		t.Fatal("undo left stale symbols")
	}
	e.Redo()
	if e.testFuncLines()[1] != "TestOmega" {
		t.Fatal("redo left stale symbols")
	}
	storage := &e.symbolCache.symbols[0]
	e.filePath = "cache.go"
	if len(e.testFuncLines()) != 0 {
		t.Fatal("changing filename left test-only gutter markers visible")
	}
	e.filePath = "renamed_test.go"
	if e.testFuncLines()[1] != "TestOmega" || &e.symbolCache.symbols[0] != storage {
		t.Fatal("test filename change did not reuse the current parse")
	}
	e.SetText(changed)
	e.MarkSaved()
	e.ParseSymbols()
	if &e.symbolCache.symbols[0] != storage {
		t.Fatal("same content/save marker invalidated parsed symbols")
	}
	// SDK low-level splices publish their text revision lazily on read.
	e.cursorLine, e.cursorCol = 1, 5
	e.insertTextAtCursor("X")
	if len(e.testFuncLines()) != 0 || e.ParseSymbols()[0].Name != "XTestOmega" {
		t.Fatal("direct splice was not observed by the symbol cache")
	}
}

func TestCodeSymbolCacheCurrentFunctionMatchesLinearSearch(t *testing.T) {
	symbols := []CodeSymbol{
		{Name: "T", Kind: SymType, Line: 1},
		{Name: "alpha", Kind: SymFunc, Line: 4},
		{Name: "v", Kind: SymVar, Line: 5},
		{Name: "beta", Kind: SymMethod, Line: 10},
		{Name: "omega", Kind: SymFunc, Line: 20},
	}
	var cache codeSymbolCache
	cache.load(codeSymbolCacheKey{}, nil, func([]string) []CodeSymbol { return symbols })
	for line := -1; line < 24; line++ {
		var want *CodeSymbol
		for i := range symbols {
			s := &symbols[i]
			if (s.Kind == SymFunc || s.Kind == SymMethod) && s.Line <= line {
				want = s
			}
		}
		if got := cache.functionAt(line); !reflect.DeepEqual(got, want) {
			t.Fatalf("functionAt(%d) = %v, want %v", line, got, want)
		}
	}
}

func TestCodeEditorSymbolsCacheDrawQueriesAllocateNothing(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("package p\n" + strings.Repeat("// cached\n", 2048) + "func TestAlpha() {}\n")
	e.filePath = "cache_test.go"
	e.cursorLine = 2049
	e.currentFunction()
	e.testFuncLines()
	if n := testing.AllocsPerRun(100, func() {
		_ = e.currentFunction()
		_ = e.testFuncLines()
	}); n != 0 {
		t.Fatalf("unchanged breadcrumb/gutter queries allocated %v times", n)
	}
}
