package prop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uk0/silk/core"
)

func TestPropertyConfigurationUserOverlay(t *testing.T) {
	root := t.TempDir()
	resource := filepath.Join(root, "resources", "property", "default.cml")
	user := filepath.Join(root, "user", "property", "default.cml")
	doc := core.NewTDoc()
	if err := doc.WriteAttr("origin", "resource"); err != nil {
		t.Fatal(err)
	}
	if err := savePropertyConfigFile(doc, resource); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(resource)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loadPropertyConfigFile(user, resource)
	if err != nil {
		t.Fatal(err)
	}
	var origin string
	if err := loaded.ReadAttr("origin", &origin); err != nil || origin != "resource" {
		t.Fatalf("resource defaults: %q, %v", origin, err)
	}
	if _, err := os.Stat(user); !os.IsNotExist(err) {
		t.Fatalf("reading defaults created user file: %v", err)
	}
	if err := loaded.WriteAttr("origin", "user"); err != nil {
		t.Fatal(err)
	}
	if err := savePropertyConfigFile(loaded, user); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadPropertyConfigFile(user, resource)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.ReadAttr("origin", &origin); err != nil || origin != "user" {
		t.Fatalf("user override: %q, %v", origin, err)
	}
	after, err := os.ReadFile(resource)
	if err != nil || string(before) != string(after) {
		t.Fatalf("resource defaults were modified: %v", err)
	}
}

func TestPropertyConfigurationInvalidNames(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", `..\escape`, "/absolute", "C:drive", "trailing."} {
		if validPropertyConfigName(name) {
			t.Errorf("accepted invalid portable file name %q", name)
		}
		pf := &PropertyConfigFile{name: name, doc: core.NewTDoc()}
		if err := pf.Save(); err == nil {
			t.Errorf("saved invalid property config %q", name)
		}
	}
}

func TestPropertyConfigurationWriteErrors(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(parent, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := savePropertyConfigFile(core.NewTDoc(), filepath.Join(parent, "default.cml")); err == nil {
		t.Fatal("save swallowed invalid parent error")
	}
}
