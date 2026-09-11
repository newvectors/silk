package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/uk0/silk/decl"
)

func TestDefaultBuildNameAcceptsPortableAndUnicodeFilenames(t *testing.T) {
	for input, want := range map[string]string{
		"main.silkui": "BuildMain", "Editor.silkui": "BuildEditor", "main-window.silkui": "BuildMainWindow", "main window.silkui": "BuildMainWindow", "9-panel.silkui": "Build9Panel", "设置面板.silkui": "Build设置面板", "main_ui.silkui": "BuildMain_ui", "---.silkui": "BuildForm",
	} {
		if got := defaultBuildName(input); got != want {
			t.Errorf("%q => %q, want %q", input, got, want)
		}
		if _, err := generateSource(input, "ui", "", decl.Form()); err != nil {
			t.Errorf("generate %q: %v", input, err)
		}
	}
}

func TestGenerateSourceRejectsInvalidIdentifiers(t *testing.T) {
	for _, pair := range [][2]string{{"foo-bar", "Build"}, {"type", "Build"}, {"_", "Build"}, {"ui", "bad-name"}, {"ui", "init"}, {"ui", "decl"}, {"ui", "_"}, {"main", "main"}} {
		if _, err := generateSource("file.silkui", pair[0], pair[1], decl.Form()); err == nil {
			t.Errorf("accepted package/function %q/%q", pair[0], pair[1])
		}
	}
}

func TestGenerateSourceIsDeterministicAndKeepsFilenameInComment(t *testing.T) {
	input := "main\npackage injected\n.silkui"
	one, err := generateSource(input, "ui", "BuildMain", decl.Form())
	if err != nil {
		t.Fatal(err)
	}
	two, err := generateSource(input, "ui", "BuildMain", decl.Form())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one, two) {
		t.Fatal("generation not deterministic")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", one, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if file.Name.Name != "ui" || len(file.Imports) != 1 || file.Imports[0].Path.Value != `"github.com/uk0/silk/decl"` {
		t.Fatalf("filename changed package/imports: %s", one)
	}
	if !strings.Contains(string(one), `\npackage injected\n`) {
		t.Fatalf("filename not quoted in comment: %s", one)
	}
}
