package silkgen

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/uk0/silk/decl"
)

func TestSourceNamesAndCommentIsolation(t *testing.T) {
	for _, tc := range []struct{ input, function string }{
		{"Editor.silkui", "BuildEditor"},
		{"sdk-platforms.silkui", "BuildSdkPlatforms"},
		{"设置-面板.silkui", "Build设置面板"},
		{"9 panel.silkui", "Build9Panel"},
		{"---.silkui", "BuildForm"},
		{"main\npackage injected\n.silkui", "BuildMainPackageInjected"},
	} {
		if got := DefaultBuildName(tc.input); got != tc.function {
			t.Errorf("%q generated %q, want %q", tc.input, got, tc.function)
		}
		one, err := GenerateSource(tc.input, "ui", "", decl.Form())
		if err != nil {
			t.Fatal(err)
		}
		two, err := GenerateSource(tc.input, "ui", tc.function, decl.Form())
		if err != nil || !bytes.Equal(one, two) {
			t.Fatalf("default/explicit generation differs: %v", err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), "generated.go", one, parser.ParseComments)
		if err != nil || file.Name.Name != "ui" || len(file.Imports) != 1 {
			t.Fatalf("invalid complete source: %v\n%s", err, one)
		}
		if !strings.Contains(string(one), "func "+tc.function+"() *decl.Node") {
			t.Fatalf("missing safe function name:\n%s", one)
		}
	}
}

func TestSourceRejectsInvalidInputs(t *testing.T) {
	for _, tc := range [][2]string{
		{"bad-package", "Build"}, {"type", "Build"}, {"_", "Build"},
		{"ui", "bad-name"}, {"ui", "init"}, {"ui", "decl"}, {"ui", "_"}, {"main", "main"},
	} {
		if _, err := GenerateSource("main.silkui", tc[0], tc[1], decl.Form()); err == nil {
			t.Errorf("accepted invalid package/function %q/%q", tc[0], tc[1])
		}
	}
	if _, err := GenerateSource("main.silkui", "ui", "", nil); err == nil {
		t.Fatal("accepted nil tree")
	}
}
