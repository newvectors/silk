package decl

import (
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/paint"
)

func TestPersistedPropertyRoundTripsWithoutInterpretingTags(t *testing.T) {
	for _, raw := range []string{`""`, `"@ref:literal"`, `"按钮 \"确定\"\n第二行"`, "#13A6E380", "false", "18446744073709551615"} {
		t.Run(raw, func(t *testing.T) {
			n := Button(P("property", Persisted{Source: raw}))
			roundTrip, err := FromTDoc(ToTDoc(n))
			if err != nil {
				t.Fatal(err)
			}
			value, _ := roundTrip.PropValue("property")
			if value != (Persisted{Source: raw}) {
				t.Fatalf("serialized value drift: %#v", value)
			}
			if _, err := parser.ParseExprFrom(token.NewFileSet(), "", ToGo(roundTrip), 0); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDecodePersistedKeepsExactValuesAndRejectsMalformedData(t *testing.T) {
	type namedInt int8
	valid := []struct {
		raw  string
		want interface{}
	}{
		{`""`, ""}, {`"@ref:literal"`, "@ref:literal"}, {"false", false},
		{"127", namedInt(127)}, {"18446744073709551615", ^uint64(0)},
		{"1.25", float32(1.25)}, {"#13A6E380", paint.Color{R: 19, G: 166, B: 227, A: 128}},
	}
	for _, tc := range valid {
		got, ok := decodePersisted(tc.raw, reflect.TypeOf(tc.want))
		if !ok || !reflect.DeepEqual(got.Interface(), tc.want) {
			t.Errorf("decode %q = %v, valid=%v; want %v", tc.raw, got, ok, tc.want)
		}
	}
	invalid := []struct {
		raw string
		typ interface{}
	}{
		{"#zzA6E380", paint.Color{}}, {"#13A6E", paint.Color{}}, {"128", namedInt(0)},
		{"-1", uint(0)}, {"NaN", float64(0)}, {"+Inf", float64(0)}, {"12junk", 0},
		{"unquoted", ""}, {"[unquoted]", []string{}},
	}
	for _, tc := range invalid {
		if _, ok := decodePersisted(tc.raw, reflect.TypeOf(tc.typ)); ok {
			t.Errorf("accepted malformed %q as %T", tc.raw, tc.typ)
		}
	}
}

func TestDesignerDecoderRejectsBadBoundsAndChildren(t *testing.T) {
	for _, bad := range []string{"not-a-rect", "[0 0 -1 10]", "[0 0 NaN 10]"} {
		doc := core.NewTDoc()
		doc.SetValue("form")
		doc.WriteAttr("bounds", bad)
		if _, err := FromTDoc(doc); err == nil {
			t.Errorf("accepted invalid designer bounds %q", bad)
		}
	}
	doc := core.NewTDoc()
	doc.SetValue("form")
	children := core.NewTDoc()
	children.SetKey("children")
	children.AddChild(core.NewTDoc())
	doc.AddChild(children)
	if _, err := FromTDoc(doc); err == nil {
		t.Fatal("silently dropped malformed child")
	}
}

func TestSDKStringItemsDecodeIsDataOnly(t *testing.T) {
	items := []string{"", "quote \" and line\nbreak", "中文", "@ref:literal"}
	raw, err := core.PersistString(items)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decodePersisted(raw, reflect.TypeOf([]string{}))
	if !ok || !reflect.DeepEqual(got.Interface(), items) {
		t.Fatalf("string items failed lossless decode: %v valid=%v raw=%s", got, ok, raw)
	}
	if got, ok := decodeStringItems("[]"); !ok || len(got) != 0 {
		t.Fatal("empty items rejected")
	}
	for _, raw := range []string{`[call()]`, `["ok"] trailing`, `["a""b"]`, `["unterminated]`, `["ok", "comma"]`, `nil`, `[123]`} {
		if _, ok := decodeStringItems(raw); ok {
			t.Errorf("accepted malformed/non-string items: %q", raw)
		}
	}
}

func TestSDKRGBATypeValidationRejectsLookalikesAndPreservesAlpha(t *testing.T) {
	type Color struct{ R, G, B, A uint8 }
	for _, value := range []interface{}{Color{}, struct{ R, G, B, A uint8 }{}, struct{ R, G, B, A uint16 }{}, &paint.Color{}} {
		typ := reflect.TypeOf(value)
		if isPaintRGBAType(typ) {
			t.Errorf("accepted non-paint RGBA type %v", typ)
		}
		if _, ok := decodePersisted("#13A6E300", typ); ok {
			t.Errorf("decoded RGBA into non-paint type %v", typ)
		}
	}
	for _, alpha := range []uint8{0, 1, 128, 255} {
		original := paint.Color{R: 19, G: 166, B: 227, A: alpha}
		raw, ok := literalPropertySource(original)
		if !ok {
			t.Fatal("real paint.Color rejected")
		}
		value, ok := decodePersisted(raw, reflect.TypeOf(original))
		if !ok || value.Interface() != original {
			t.Fatalf("8-bit sRGB/alpha changed: %v -> %s -> %v", original, raw, value)
		}
	}
}
