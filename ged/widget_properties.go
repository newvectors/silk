package ged

import (
	"reflect"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/decl"
	"github.com/uk0/silk/paint"
)

// Embedded widget properties are separate from FakeWidget geometry, names,
// event handlers and children. Persist scalars, exact paint.Color and []string
// item lists exposed by the property sheet; skip other complex types. Values
// use TDoc's core.PersistString syntax, not executable expressions or tag refs.
func captureWidgetProperties(w core.IEnumProperties) map[string]string {
	c := &propCapture{vals: make(map[string]string)}
	w.EnumProperties(c)
	return c.vals
}

// Use the SDK's exact codec and dependency handling for the designer, its live
// preview and clipboard/morph restoration too. Sequential scalar setters used
// to lose item lists and clamp values against partially restored ranges.
func applyWidgetProperties(w core.IEnumProperties, vals map[string]string) {
	decl.ApplyPersistedProperties(w, vals)
}

type propCapture struct {
	vals map[string]string
}

func (c *propCapture) AddProperty(id string, get, set interface{}) {
	if get == nil || set == nil {
		return
	}
	gv := reflect.ValueOf(get)
	gt := gv.Type()
	if gt.Kind() != reflect.Func || gv.IsNil() || gt.NumIn() != 0 || gt.NumOut() != 1 {
		return
	}
	if s, ok := scalarToString(gv.Call(nil)[0]); ok {
		c.vals[id] = s
	}
}

var colorType = reflect.TypeOf(paint.Color{})

// Normalize named scalar kinds before persistence. Only the exact supported
// complex types are accepted, rather than capturing data we cannot restore.
func scalarToString(v reflect.Value) (string, bool) {
	if v.Type() == colorType || v.Type() == reflect.TypeOf([]string{}) {
		s, err := core.PersistString(v.Interface())
		return s, err == nil
	}
	var raw interface{}
	switch v.Kind() {
	case reflect.String:
		raw = v.String()
	case reflect.Bool:
		raw = v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		raw = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		raw = v.Uint()
	case reflect.Float32, reflect.Float64:
		raw = v.Float()
	default:
		return "", false
	}
	s, err := core.PersistString(raw)
	return s, err == nil
}
