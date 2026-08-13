package gui

import (
	"github.com/uk0/silk/core"
)

// SetpointBox 设定值输入框: 操作员改写工艺设定值并回写到 tag 的控件。
//
// It embeds NumberInput, so the value, the [min,max] clamp, the keyboard
// editing and the step buttons are the ones the operator already knows. What it
// adds is the write direction the rest of the 组态 widgets do not have: a
// commit callback fired on Enter and on blur, which scada.BindScreen routes to
// core.Tag.SetValue.
//
// SetValue deliberately does NOT commit. The tag->widget refresh drives
// SetValue on every sample, so committing from there would write each displayed
// value straight back to the tag.
type SetpointBox struct {
	NumberInput
	tagName string // design-time tag name this setpoint reads and writes

	cbCommit func(float64)
}

func init() {
	core.RegisterFactory("gui.SetpointBox", core.TypeOf((*SetpointBox)(nil)))
}

// Compile-time checks: SetpointBox must satisfy the keyboard and text-input
// event interfaces the window dispatches (window_glfw.go onKey/onChar), or the
// Enter that commits a setpoint silently never reaches it.
var (
	_ IEventKeyDown   = (*SetpointBox)(nil)
	_ IEventTextInput = (*SetpointBox)(nil)
)

// NewSetpointBox creates a setpoint field with the NumberInput defaults
// (range 0..100, step 1, 2 decimals) and no tag.
func NewSetpointBox() *SetpointBox {
	p := new(SetpointBox)
	p.Init(p)
	return p
}

// SigCommit installs the operator-confirm callback — the one place a value
// leaves this widget. Pass nil to detach it.
func (this *SetpointBox) SigCommit(fn func(float64)) {
	this.cbCommit = fn
}

// Commit hands the current value to the commit callback, if one is installed.
func (this *SetpointBox) Commit() {
	if this.cbCommit != nil {
		this.cbCommit(this.Value())
	}
}

// OnKeyDown lets the embedded NumberInput handle the key first — Enter there
// closes an open edit session and parses the typed text into Value() — then
// commits, so what is written is the value the operator is reading.
func (this *SetpointBox) OnKeyDown(key int, repeat bool) {
	this.NumberInput.OnKeyDown(key, repeat)
	if key == KeyEnter {
		this.Commit()
	}
}

// OnFocusOut commits what the operator left in the field, after NumberInput's
// own blur handling has committed any open edit into Value().
func (this *SetpointBox) OnFocusOut() {
	this.NumberInput.OnFocusOut()
	this.Commit()
}

// SetTagName sets the design-time tag name that this widget reads and writes.
func (this *SetpointBox) SetTagName(s string) {
	this.tagName = s
	this.Self().Update()
}

// TagName returns the design-time tag name.
func (this *SetpointBox) TagName() string { return this.tagName }

func (this *SetpointBox) EnumProperties(list core.IPropertyList) {
	this.NumberInput.EnumProperties(list)
	list.AddProperty("tag", this.TagName, this.SetTagName)
}
