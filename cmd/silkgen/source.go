package main

import (
	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/internal/silkgen"
)

func generateSource(input, packageName, functionName string, tree *decl.Node) ([]byte, error) {
	return silkgen.GenerateSource(input, packageName, functionName, tree)
}

// Keep already-valid stems stable, but treat punctuation/spaces as word
// boundaries. "main-window.silkui" must not produce invalid BuildMain-window.
func defaultBuildName(input string) string {
	return silkgen.DefaultBuildName(input)
}
