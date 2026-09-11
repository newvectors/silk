package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/internal/silkgen"
)

func TestRegenerateGoForPortableUnicodeFilenames(t *testing.T) {
	for _, tc := range []struct{ stem, function string }{
		{"sdk-platforms", "BuildSdkPlatforms"},
		{"设置-面板", "Build设置面板"},
		{"9 control panel", "Build9ControlPanel"},
	} {
		t.Run(tc.stem, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), tc.stem+".silkui")
			tree := decl.Form(decl.ID("root"), decl.Child(decl.Button(decl.ID("action"), decl.P("text", "确认"))))
			if err := decl.ToTDoc(tree).SaveFile(input); err != nil {
				t.Fatal(err)
			}
			regenerateGoForSilkui(input)
			output := filepath.Join(filepath.Dir(input), tc.stem+".silk.go")
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatalf("IDE failed to generate %q: %v", input, err)
			}
			file, err := parser.ParseFile(token.NewFileSet(), output, got, 0)
			if err != nil {
				t.Fatal(err)
			}
			if file.Name.Name != "ui" || len(file.Imports) != 1 || file.Imports[0].Path.Value != `"github.com/uk0/silk/decl"` {
				t.Fatalf("incorrect package/imports:\n%s", got)
			}
			function, ok := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			if !ok || function.Name.Name != tc.function {
				t.Fatalf("incorrect generated function, want %s:\n%s", tc.function, got)
			}
			want, err := silkgen.GenerateSource(input, "ui", "", tree)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("IDE diverged from shared CLI generation: %v\n%s", err, got)
			}
		})
	}
}
