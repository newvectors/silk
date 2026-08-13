package ged

// A silk widget plugin is an ordinary Go package whose init() does two things:
// core.RegisterFactory names the widget type, so the designer can build one and
// load one out of a .silkui file, and ged.RegisterWidget writes the four tables
// that decide whether the designer can do anything with it — the codegen
// mapping (factoryMap), the palette category (categoryDefs), the bindable
// events (widgetEvents) and the auto-default event (pluginDefaultEvents). One
// call writes all four, because a widget in only some of them is a widget the
// designer half-supports: in the palette but generated as core.New(...), or
// generatable but unreachable, both of which the drift tests in
// ged/codegen_registry_test.go already fail on.
//
// Linking is the whole distribution story: the designer binary must import the
// plugin package for its init() to run. There is no dynamic loading and none is
// coming. Nothing in the tree calls plugin.Open; Go's plugin buildmode does not
// work on Windows, one of the three targets silk builds for; and the yaegi
// script engine exposes only the tag API and a stdlib subset
// (Engine.silkSymbols, script/engine.go:71) — neither core nor gui — so a
// script cannot register a factory either.
//
// A design naming a widget whose plugin is not linked in is not lost, it
// degrades down the path that already exists for a widget renamed or removed
// across versions: loadChildWidgets (ged/fake-widget.go:612) skips the node and
// its whole subtree and records the factory name, GedScene.MissingWidgets
// (ged/ged-scene.go:318) carries the names out, LoadProblems
// (ged/design-problems.go:79) turns each into a 问题 row, and 预览/导出/运行 are
// cancelled before any of them can write a file the widget is silently missing
// from (design.go:641).

import (
	"strings"

	"github.com/uk0/silk/core"
)

// WidgetEvent is one bindable event of a plugin widget: the exported mirror of
// widgetEvent (ged/codegen.go), whose fields a package outside ged cannot fill.
type WidgetEvent struct {
	Name   string // event name, as stored in the design and shown in the property sheet
	Code   string // formatted with (fieldName, handlerName); ends in a newline
	Params string // the handler's Go parameter list, "" for none
	Import string // extra import the emitted line references, "" for none
}

// WidgetDef is everything the designer has to know about a widget it did not
// declare itself: how to generate it (GoType/ImportPath/Constructor), where the
// palette offers it (Category), and what it can bind (Events/DefaultEvent).
type WidgetDef struct {
	Factory      string // core factory name, e.g. "acme.Meter"; must already be registered
	Category     string // palette category display name; a new one is appended when unknown
	GoType       string // generated field type, e.g. "*acme.Meter"
	ImportPath   string // import the generated file needs for GoType/Constructor
	Constructor  string // constructor expression, e.g. `acme.NewMeter()`
	Events       []WidgetEvent
	DefaultEvent string // event auto-bound when the user writes one handler func, "" for none
}

// pluginDefaultEvents holds the auto-default event of each registered plugin
// widget. defaultEventForFactory (ged/codegen.go) reads it before its own
// switch, which is the built-in widgets' half of the same table.
var pluginDefaultEvents = map[string]string{}

// RegisterWidget adds one plugin widget to every table the designer gates on.
// It is the only way into them from outside package ged, and it is
// all-or-nothing: the def is validated in full before anything is written, so a
// rejected one leaves the tables exactly as they were rather than half-written.
//
// Call it from the plugin package's init(), after core.RegisterFactory has
// registered d.Factory.
func RegisterWidget(d WidgetDef) error {
	for _, f := range []struct{ name, value string }{
		{"Factory", d.Factory},
		{"Category", d.Category},
		{"GoType", d.GoType},
		{"ImportPath", d.ImportPath},
		{"Constructor", d.Constructor},
	} {
		if strings.TrimSpace(f.value) == "" {
			return core.StrErr("RegisterWidget: " + f.name + " is empty")
		}
	}
	if core.FindFactory(d.Factory) == nil {
		return core.StrErr(`RegisterWidget: Factory "` + d.Factory +
			`" is not registered; call core.RegisterFactory first`)
	}
	if _, dup := factoryMap[d.Factory]; dup {
		return core.StrErr(`RegisterWidget: Factory "` + d.Factory + `" is already registered`)
	}
	if d.DefaultEvent != "" {
		known := false
		for _, e := range d.Events {
			if e.Name == d.DefaultEvent {
				known = true
				break
			}
		}
		if !known {
			return core.StrErr(`RegisterWidget: DefaultEvent "` + d.DefaultEvent +
				`" is not one of Events`)
		}
	}
	for _, e := range d.Events {
		if strings.TrimSpace(e.Name) == "" {
			return core.StrErr("RegisterWidget: Events has an entry with an empty Name")
		}
		if strings.TrimSpace(e.Code) == "" {
			return core.StrErr(`RegisterWidget: Events entry "` + e.Name + `" has an empty Code`)
		}
	}

	factoryMap[d.Factory] = widgetMapping{
		goType:      d.GoType,
		importPath:  d.ImportPath,
		constructor: d.Constructor,
	}

	categorised := false
	for i := range categoryDefs {
		if categoryDefs[i].name == d.Category {
			categoryDefs[i].names = append(categoryDefs[i].names, d.Factory)
			categorised = true
			break
		}
	}
	if !categorised {
		categoryDefs = append(categoryDefs, widgetCategoryDef{
			name:  d.Category,
			names: []string{d.Factory},
		})
	}

	if len(d.Events) > 0 {
		evts := make([]widgetEvent, 0, len(d.Events))
		for _, e := range d.Events {
			evts = append(evts, widgetEvent{name: e.Name, code: e.Code, imp: e.Import, params: e.Params})
		}
		widgetEvents[d.Factory] = evts
	}
	if d.DefaultEvent != "" {
		pluginDefaultEvents[d.Factory] = d.DefaultEvent
	}
	return nil
}
