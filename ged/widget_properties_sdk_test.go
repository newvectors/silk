package ged

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/gui"
)

func TestDesignerPropertiesRestoreListsBeforeSelection(t *testing.T) {
	items := []string{"Windows", "Linux \"桌面\"\n第二行", "macOS", "", "@ref:literal"}
	src := gui.NewComboBox()
	src.SetItems(items)
	src.SetActiveIndex(2)
	vals := captureWidgetProperties(src)
	if vals["items"] == "" {
		t.Fatal("designer drops the ComboBox item list while saving")
	}
	dst := gui.NewComboBox()
	applyWidgetProperties(dst, vals)
	if !reflect.DeepEqual(dst.Items(), items) || dst.ActiveIndex() != 2 || dst.Text() != "macOS" {
		t.Fatalf("restored list/selection mismatch: %q, index=%d, text=%q", dst.Items(), dst.ActiveIndex(), dst.Text())
	}
	// A deliberately empty list must clear existing factory/custom contents.
	applyWidgetProperties(dst, map[string]string{"items": "[]", "selectedIndex": "-1", "text": `""`})
	if len(dst.Items()) != 0 || dst.ActiveIndex() != -1 || dst.Text() != "" {
		t.Fatal("empty list/selection was treated as a missing property")
	}
}

func TestDesignerPropertiesRestoreDependentTuplesAtomically(t *testing.T) {
	slider := gui.NewSlider(0, 100)
	applyWidgetProperties(slider, map[string]string{"值": "-150", "最小值": "-200", "最大值": "-100"})
	if slider.Min() != -200 || slider.Max() != -100 || slider.Value() != -150 {
		t.Fatalf("slider restored value before its range: [%v,%v] value=%v", slider.Min(), slider.Max(), slider.Value())
	}
	date := gui.NewDatePicker()
	date.SetDate(2024, 2, 29)
	applyWidgetProperties(date, map[string]string{"Year": "2025", "Month": "1", "Day": "29", "Format": `"02/01/2006"`})
	if date.Year() != 2025 || date.Month() != 1 || date.Day() != 29 || date.Format() != "02/01/2006" {
		t.Fatalf("date was clamped against an intermediate non-leap February: %d-%d-%d %q", date.Year(), date.Month(), date.Day(), date.Format())
	}
}

type reverseComboProperties struct{ *gui.ComboBox }

func (c reverseComboProperties) EnumProperties(list core.IPropertyList) {
	list.AddProperty("selectedIndex", c.ActiveIndex, c.SetActiveIndex)
	list.AddProperty("text", c.Text, c.SetText)
	list.AddProperty("items", c.Items, c.SetItems)
}

type readOnlyComboItems struct{ *gui.ComboBox }

func (c readOnlyComboItems) EnumProperties(list core.IPropertyList) {
	list.AddProperty("items", c.Items, nil)
	list.AddProperty("text", c.Text, c.SetText)
}

func TestDesignerPropertiesDependencyOrderingAndAllowlist(t *testing.T) {
	combo := gui.NewComboBox()
	applyWidgetProperties(reverseComboProperties{combo}, map[string]string{
		"items": `["Windows" "Linux" "macOS"]`, "selectedIndex": "2", "text": `"macOS"`,
	})
	if combo.ActiveIndex() != 2 || combo.Text() != "macOS" || len(combo.Items()) != 3 {
		t.Fatal("items dependency must not depend on property enumeration order")
	}
	for _, bad := range []string{`[call()]`, `["ok"] trailing`, `["a""b"]`, `["ok", "comma"]`} {
		applyWidgetProperties(combo, map[string]string{"items": bad})
		if combo.ActiveIndex() != 2 || len(combo.Items()) != 3 {
			t.Fatalf("malformed item list %q mutated the widget", bad)
		}
	}
	applyWidgetProperties(readOnlyComboItems{combo}, map[string]string{"items": "[]", "text": `"custom"`})
	if len(combo.Items()) != 3 || combo.Text() != "custom" {
		t.Fatal("compound setters must respect the explicit writable-property allowlist")
	}
}

func TestDesignerComboSaveLoadAndPreviewKeepItems(t *testing.T) {
	fw, err := NewFakeWidgetFromFactory("gui.ComboBox")
	if err != nil {
		t.Fatal(err)
	}
	items := []string{"Windows", "Linux \"桌面\"\n第二行", "macOS"}
	raw, err := core.PersistString(items)
	if err != nil {
		t.Fatal(err)
	}
	doc := core.NewTDoc()
	doc.SetValue("gui.ComboBox")
	doc.WriteAttr("name", "sdkPlatform")
	props := core.NewTDoc()
	props.SetKey("props")
	doc.AddChild(props)
	for _, p := range [][2]string{{"selectedIndex", "2"}, {"text", strconv.Quote("macOS")}, {"items", raw}} {
		props.WriteAttr(p[0], p[1])
	}
	for round := 0; round < 3; round++ {
		if err := fw.LoadDesign(doc); err != nil {
			t.Fatal(err)
		}
		for phase, object := range map[string]gui.IWidget{"designer": fw.Widget(), "preview": fw.Generate()} {
			combo := object.(*gui.ComboBox)
			if !reflect.DeepEqual(combo.Items(), items) || combo.ActiveIndex() != 2 || combo.Text() != "macOS" {
				t.Fatalf("round %d %s: items=%q, index=%d, text=%q", round, phase, combo.Items(), combo.ActiveIndex(), combo.Text())
			}
		}
		doc, err = core.LoadTDocStr(fw.SaveDesign().String())
		if err != nil {
			t.Fatal(err)
		}
		var saved string
		if err := doc.ChildByKey("props", false).ReadAttr("items", &saved); err != nil || saved != raw {
			t.Fatalf("list lost or escaped incorrectly: %q, want %q, err=%v", saved, raw, err)
		}
	}
}
