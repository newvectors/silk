package decl

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/geom"
)

// TDoc layout produced/consumed by this codec
//
//	root: val=<factory name>          // e.g. "gui.Form"
//	  "name": val=<id string>         // optional, only when Node.ID != ""
//	  <prop name>: val=<encoded value>
//	  ...
//	  "widget": <recursive subtree>
//	  ...
//
// The "widget" key for children matches the modern dialect of formloader.go,
// so a Node serialised by this codec is loadable by gui.LoadFormFromDoc
// without any glue. The "name" key matches what the designer writes for
// the widget identifier.
//
// Value encoding (string scope): the four Value variants ride on top of
// the TDoc's persisted-string format. Lit primitives go through TDoc's
// own typed accessors (Value/SetValue use the persist layer). Ref / Bind /
// Expr encode as tag-prefixed strings so a single TDoc string can carry
// any of them:
//
//	"@ref:OnClick"      → Ref{Name: "OnClick"}
//	"@bind:user.name"   → Bind{Path: "user.name"}
//	"@expr:foo()*2"     → Expr{Source: "foo()*2"}
//
// A literal string that happens to start with "@ref:" is theoretically
// ambiguous; we accept that — designs that want a literal "@ref:..." can
// double-prefix with "@lit:".

// Tag prefixes used to distinguish non-literal value variants in the
// serialised string scope.
const (
	tagPersisted = "@persist:" // typed designer property, not a literal/tag expression
	tagRef       = "@ref:"
	tagBind      = "@bind:"
	tagExpr      = "@expr:"
	tagTr        = "@tr:"  // i18n translation key
	tagLit       = "@lit:" // explicit literal escape for strings starting with "@"
)

// ToTDoc serialises n into a *core.TDoc rooted at the returned doc. The
// returned doc has no parent and can be directly fed to TDocFile.WriteTo
// or further nested under a parent designer document.
//
// nil n returns nil. Internal nil children are skipped silently — the
// builder allows passing nil into Children(...), and we don't want every
// emitter to defensively filter.
func ToTDoc(n *Node) *core.TDoc {
	if n == nil {
		return nil
	}
	doc := core.NewTDoc()
	_ = doc.SetValue(n.Type)

	if n.ID != "" {
		_ = doc.WriteAttr("name", n.ID)
	}

	for _, p := range n.Props {
		writeProp(doc, p.Name, p.Value)
	}

	for _, c := range n.Children {
		if c == nil {
			continue
		}
		sub := ToTDoc(c)
		if sub == nil {
			continue
		}
		sub.SetKey("widget")
		doc.AddChild(sub)
	}

	return doc
}

// FromTDoc reads both the declarative dialect (gui.Form/widget) and the
// designer's persisted dialect (form/children/props). Designer bounds are in
// millimetres and become logical pixels at 96 DPI, independent of the machine
// running codegen. Property-sheet values retain their exact serialized form.
// Unknown factories survive parsing and fail explicitly when Build is called.
func FromTDoc(doc *core.TDoc) (*Node, error) {
	return fromTDoc(doc, false)
}

func fromTDoc(doc *core.TDoc, localBounds bool) (*Node, error) {
	if doc == nil {
		return nil, errors.New("decl.FromTDoc: nil doc")
	}
	var typeName string
	_ = doc.Value(&typeName)
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		_ = doc.ReadAttr("factory", &typeName)
	}
	if typeName == "" {
		_ = doc.ReadAttr("factoryName", &typeName)
	}
	typeName = strings.TrimSpace(typeName)
	var boundsSpace string
	_ = doc.ReadAttr("bounds_space", &boundsSpace)
	localBounds = localBounds || boundsSpace == "local"
	if typeName == "" {
		return nil, errors.New("decl.FromTDoc: empty factory name (TDoc value)")
	}

	designerRoot := typeName == "form"
	if designerRoot {
		typeName = "gui.Form"
	}
	n := &Node{Type: typeName}
	var sharedChildren []*Node

	for _, sub := range doc.Childdren() {
		key := sub.Key()
		switch key {
		case "":
			// Unkeyed sub-nodes — legacy designer dialect put children here.
			// We accept them as nested widgets to keep the loader symmetric
			// with formloader.LoadFormFromDoc's "children" branch.
			if sub.HasValue() || sub.HasChildren() {
				child, err := fromTDoc(sub, localBounds)
				if err != nil {
					return nil, err
				}
				n.Children = append(n.Children, child)
				if sub.ChildByKey("bounds", false) != nil {
					sharedChildren = append(sharedChildren, child)
				}
			}
		case "children":
			for _, childDoc := range sub.Childdren() {
				child, err := fromTDoc(childDoc, localBounds)
				if err != nil {
					return nil, fmt.Errorf("decl.FromTDoc: %s child: %w", typeName, err)
				}
				n.Children = append(n.Children, child)
				if childDoc.ChildByKey("bounds", false) != nil {
					sharedChildren = append(sharedChildren, child)
				}
			}
		case "bounds":
			var bounds geom.Rect
			if err := sub.Value(&bounds); err != nil {
				return nil, fmt.Errorf("decl.FromTDoc: invalid bounds: %w", err)
			}
			for _, p := range []struct {
				name string
				mm   float64
			}{{"x", bounds.X}, {"y", bounds.Y}, {"width", bounds.Width}, {"height", bounds.Height}} {
				if math.IsNaN(p.mm) || math.IsInf(p.mm, 0) || ((p.name == "width" || p.name == "height") && p.mm < 0) {
					return nil, fmt.Errorf("decl.FromTDoc: invalid %s in bounds", p.name)
				}
				n.SetProp(p.name, Lit{V: math.Round(p.mm * 96 / 25.4)})
			}
		case "props":
			for _, propDoc := range sub.Childdren() {
				var raw string
				if err := propDoc.Value(&raw); err != nil {
					return nil, fmt.Errorf("decl.FromTDoc: property %s: %w", propDoc.Key(), err)
				}
				n.SetProp(propDoc.Key(), Persisted{Source: raw})
			}
		case "factory", "factoryName", "bounds_space":
			// Structural compatibility metadata, not widget properties.
		case "name":
			var s string
			_ = sub.Value(&s)
			n.ID = s
		case "widget":
			child, err := fromTDoc(sub, localBounds)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		default:
			// Unlike Value(&string), which stops at whitespace for unquoted
			// tokens, this scanner retains an entire native TDoc string list.
			var raw tdocLiteralSource
			if err := sub.Value(&raw); err == nil {
				if items, ok := decodeStringItems(string(raw)); ok {
					n.Props = append(n.Props, Prop{Name: key, Value: Lit{V: items}})
					continue
				}
			}
			// Property: read its serialised string and decode the variant.
			var s string
			_ = sub.Value(&s)
			n.Props = append(n.Props, Prop{Name: key, Value: decodeValue(s)})
		}
	}

	// Each recursive child still carries its own absolute origin here; only
	// its descendants have been localized. Convert it once at this boundary.
	if !localBounds && !designerRoot && doc.ChildByKey("bounds", false) != nil {
		for _, child := range sharedChildren {
			for _, axis := range []string{"x", "y"} {
				pv, pok := n.PropValue(axis)
				cv, cok := child.PropValue(axis)
				pl, plok := pv.(Lit)
				cl, clok := cv.(Lit)
				if pok && cok && plok && clok {
					p, pvalid := asFloat(pl.V)
					c, cvalid := asFloat(cl.V)
					if pvalid && cvalid {
						child.SetProp(axis, Lit{V: c - p})
					}
				}
			}
		}
	}
	if doc.ChildByKey("bounds", false) != nil {
		switch typeName {
		case "gui.HBox", "gui.VBox", "gui.GridLayout", "gui.FormLayout":
			enabled := false
			doc.ReadAttr("layout_managed", &enabled)
			n.SetProp("layout_enabled", Lit{V: enabled})
		}
	}
	// The SDK's older scalar aliases are millimetres; the declarative
	// width/height dialect is already in logical pixels. Never mix the two.
	aliases := map[string]string{}
	if doc.ChildByKey("bounds", false) == nil {
		if designerRoot {
			aliases["form_w"], aliases["form_h"] = "width", "height"
		}
		if doc.ChildByKey("w", false) != nil || doc.ChildByKey("h", false) != nil {
			aliases["x"], aliases["y"], aliases["w"], aliases["h"] = "x", "y", "width", "height"
		}
	}
	keys := make([]string, 0, len(aliases))
	for source := range aliases {
		keys = append(keys, source)
	}
	sort.Strings(keys)
	for _, source := range keys {
		target := aliases[source]
		if attr := doc.ChildByKey(source, false); attr != nil {
			var number float64
			if err := attr.Value(&number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) || ((target == "width" || target == "height") && number < 0) {
				return nil, fmt.Errorf("decl.FromTDoc: invalid %s", source)
			}
			n.SetProp(target, Lit{V: math.Round(number * 96 / 25.4)})
		}
	}
	if designerRoot && doc.ChildByKey("title", false) == nil {
		if title := doc.ChildByKey("form_title", false); title != nil {
			var text string
			if err := title.Value(&text); err != nil {
				return nil, err
			}
			n.SetProp("title", Lit{V: text})
		}
	}
	return n, nil
}

// writeProp dispatches on the Value variant to choose the right TDoc
// write path. Lit values pass through WriteAttr's native typed handling
// — the persist layer formats numbers, bools, and strings (with
// quoting) without our intervention. Ref/Bind/Expr round-trip via a
// tag-prefixed string so the variant is recoverable on read.
//
// The earlier "stringify then write" approach double-encoded strings:
// PersistString quoted "OK" to `"OK"`, the resulting Go string was
// then written through SetValue which called PersistString again,
// producing `"\"OK\""` on disk. Splitting the two paths fixes that.
func writeProp(doc *core.TDoc, name string, v Value) {
	switch x := v.(type) {
	case Lit:
		// String literals that start with "@" need the explicit @lit:
		// escape so they don't parse back as Ref/Bind/Expr after a
		// round-trip. Numeric and bool primitives never collide with
		// the tag prefixes.
		if s, ok := x.V.(string); ok && strings.HasPrefix(s, "@") {
			_ = doc.WriteAttr(name, tagLit+s)
			return
		}
		_ = doc.WriteAttr(name, x.V)
	case Persisted:
		_ = doc.WriteAttr(name, tagPersisted+x.Source)
	case Ref:
		_ = doc.WriteAttr(name, tagRef+x.Name)
	case Bind:
		_ = doc.WriteAttr(name, tagBind+x.Path)
	case Expr:
		_ = doc.WriteAttr(name, tagExpr+x.Source)
	case TrKey:
		_ = doc.WriteAttr(name, tagTr+x.Source)
	}
}

// decodeValue is the inverse of writeProp. The four tag prefixes route
// directly to Ref/Bind/Expr/Lit-escape; everything else is wrapped as
// a Lit holding the deserialised Go string. Type coercion (string →
// bool / float / int) is the runtime's responsibility — applyProp's
// asBool / asFloat already handle this for the dominant prop names so
// the AST stays a faithful textual mirror of what was written.
//
// Note: the input s is assumed to have already been unquoted by
// TDoc.Value(&s), which uses PersistSscan's quotedString scanner. So
// "\"OK\"" on disk arrives here as the Go string OK; tag dispatch works
// against the user-visible value, not the on-disk encoding.
func decodeValue(s string) Value {
	switch {
	case strings.HasPrefix(s, tagPersisted):
		return Persisted{Source: s[len(tagPersisted):]}
	case strings.HasPrefix(s, tagRef):
		return Ref{Name: s[len(tagRef):]}
	case strings.HasPrefix(s, tagBind):
		return Bind{Path: s[len(tagBind):]}
	case strings.HasPrefix(s, tagExpr):
		return Expr{Source: s[len(tagExpr):]}
	case strings.HasPrefix(s, tagTr):
		return TrKey{Source: s[len(tagTr):]}
	case strings.HasPrefix(s, tagLit):
		return Lit{V: s[len(tagLit):]}
	default:
		return Lit{V: s}
	}
}

// A data-only fmt.Scanner exposes the complete TDoc literal without reaching
// into core's private val field or changing its general string scanner.
type tdocLiteralSource string

func (raw *tdocLiteralSource) Scan(state fmt.ScanState, _ rune) error {
	var b strings.Builder
	for {
		r, _, err := state.ReadRune()
		if err == io.EOF {
			*raw = tdocLiteralSource(b.String())
			return nil
		}
		if err != nil {
			return err
		}
		b.WriteRune(r)
	}
}
