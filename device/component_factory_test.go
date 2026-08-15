package device

import (
	"strings"
	"testing"

	"github.com/uk0/silk/core"
	"github.com/uk0/silk/gui"
	"github.com/uk0/silk/paint"
)

// TestFactoryDeviceComponentHasAConnection: the designer and the .tdoc loader
// build the component through the core factory, which reflects on Init and
// never runs NewDeviceComponent. With the connection defaults left in the
// constructor, the component dropped on a screen draws ":0" and Start would
// dial "tcp://:0" as unit 0.
func TestFactoryDeviceComponentHasAConnection(t *testing.T) {
	f := core.FindFactory("gui.DeviceComponent")
	if f == nil {
		t.Fatal(`no factory registered for "gui.DeviceComponent"`)
	}
	d, ok := f.New().(*DeviceComponent)
	if !ok {
		t.Fatalf("factory produced %T, want *DeviceComponent", f.New())
	}
	d.SetSize(180, 60)

	rec := &deviceRecorder{Painter: nopDevicePainter{}}
	d.Draw(rec)

	addr := ""
	for _, s := range rec.texts {
		if strings.Contains(s, ":") && !strings.HasPrefix(s, "⚙") {
			addr = s
		}
	}
	if addr != "127.0.0.1:502" {
		t.Errorf("the component draws %q as its endpoint, want \"127.0.0.1:502\" (drew %v)", addr, rec.texts)
	}
	if d.UnitID() < 1 {
		t.Errorf("unit id %d addresses no Modbus slave", d.UnitID())
	}
	if d.Period() <= 0 {
		t.Errorf("poll period %d ms", d.Period())
	}
	if d.Slot() < 1 {
		t.Errorf("S7 slot %d", d.Slot())
	}
}

// deviceRecorder captures the text DeviceComponent.Draw puts on the design
// surface. nopDevicePainter supplies the ops it doesn't use — an op it starts
// using panics here rather than passing silently.
type deviceRecorder struct {
	paint.Painter
	texts []string
}

func (r *deviceRecorder) Rectangle(x, y, w, h float64)         {}
func (r *deviceRecorder) Fill()                                {}
func (r *deviceRecorder) Stroke()                              {}
func (r *deviceRecorder) SetBrush1(c paint.Color)              {}
func (r *deviceRecorder) SetPen1(c paint.Color, width float64) {}
func (r *deviceRecorder) DrawText1(x, y float64, s string)     { r.texts = append(r.texts, s) }

type nopDevicePainter struct{ paint.Painter }

var _ gui.IWidget = (*DeviceComponent)(nil)
