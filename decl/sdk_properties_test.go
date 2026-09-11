package decl_test

import (
	"github.com/uk0/silk/core"
	"reflect"
	"testing"

	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/gui"
)

func TestSDKLiteralPropertiesReachRegisteredPropertySheet(t *testing.T) {
	tree := decl.Form(decl.Children(
		decl.New("gui.DatePicker", decl.ID("date"), decl.P("Day", 29), decl.P("Month", 2), decl.P("Year", 2024)),
		decl.ComboBox(decl.ID("combo"), decl.P("可用", false), decl.P("可见", false)),
		decl.ProgressBar(decl.ID("progress"), decl.P("显示文本", false)),
	))
	_, index, err := tree.BuildWithIndex()
	if err != nil {
		t.Fatal(err)
	}
	date := index["date"].(*gui.DatePicker)
	if date.Year() != 2024 || date.Month() != 2 || date.Day() != 29 {
		t.Errorf("literal date = %d-%02d-%02d, want 2024-02-29", date.Year(), date.Month(), date.Day())
	}
	if index["progress"].(*gui.ProgressBar).IsShowText() {
		t.Error("property-sheet literal false was ignored")
	}
	combo := index["combo"].(*gui.ComboBox)
	if combo.IsEnabled() || combo.IsVisible() {
		t.Errorf("property-sheet literal booleans ignored: enabled=%v visible=%v", combo.IsEnabled(), combo.IsVisible())
	}
}

func TestSDKRangeValueDoesNotDependOnPropertyOrder(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		for _, negative := range []bool{false, true} {
			min, max, value := 200., 300., 250.
			minName, maxName, valueName := "min", "max", "value"
			if negative {
				min, max, value = -300, -200, -250
			}
			var lo, hi, val interface{} = min, max, value
			if persisted {
				minName, maxName, valueName = "最小值", "最大值", "值"
				if negative {
					lo, hi, val = decl.Persisted{Source: "-300"}, decl.Persisted{Source: "-200"}, decl.Persisted{Source: "-250"}
				} else {
					lo, hi, val = decl.Persisted{Source: "200"}, decl.Persisted{Source: "300"}, decl.Persisted{Source: "250"}
				}
			}
			for _, order := range [][]decl.Attr{
				{decl.P(valueName, val), decl.P(minName, lo), decl.P(maxName, hi)},
				{decl.P(maxName, hi), decl.P(valueName, val), decl.P(minName, lo)},
			} {
				obj, err := decl.Slider(order...).Build()
				if err != nil {
					t.Fatal(err)
				}
				slider := obj.(*gui.Slider)
				if slider.Min() != min || slider.Max() != max || slider.Value() != value {
					t.Errorf("persisted=%v negative=%v: range/value=(%v,%v)/%v, want (%v,%v)/%v", persisted, negative, slider.Min(), slider.Max(), slider.Value(), min, max, value)
				}
			}
		}
	}
}

func TestSDKPersistedDateIsAtomicAcrossPropertyOrder(t *testing.T) {
	obj, err := decl.New("gui.DatePicker", decl.P("Day", decl.Persisted{Source: "31"}), decl.P("Month", decl.Persisted{Source: "1"}), decl.P("Year", decl.Persisted{Source: "2024"})).Build()
	if err != nil {
		t.Fatal(err)
	}
	d := obj.(*gui.DatePicker)
	if d.Year() != 2024 || d.Month() != 1 || d.Day() != 31 {
		t.Fatalf("persisted date clamped while partially applied: %d-%02d-%02d", d.Year(), d.Month(), d.Day())
	}
}

func TestSDKComboLiteralAndPersistedItemsPrecedeSelection(t *testing.T) {
	items := []string{"", "引号 \"和换行\n", "@ref:literal"}
	for _, roundtrip := range []bool{false, true} {
		for _, persisted := range []bool{false, true} {
			var values interface{} = items
			var selected interface{} = 2
			if persisted {
				raw, err := core.PersistString(items)
				if err != nil {
					t.Fatal(err)
				}
				values, selected = decl.Persisted{Source: raw}, decl.Persisted{Source: "2"}
			}
			node := decl.ComboBox(decl.P("selectedIndex", selected), decl.P("items", values))
			if roundtrip {
				var err error
				node, err = decl.FromTDoc(decl.ToTDoc(node))
				if err != nil {
					t.Fatal(err)
				}
			}
			obj, err := node.Build()
			if err != nil {
				t.Fatal(err)
			}
			combo := obj.(*gui.ComboBox)
			if !reflect.DeepEqual(combo.Items(), items) || combo.ActiveIndex() != 2 || combo.Text() != items[2] {
				t.Fatalf("roundtrip=%v persisted=%v: items=%v index=%d text=%q", roundtrip, persisted, combo.Items(), combo.ActiveIndex(), combo.Text())
			}
		}
	}
}
