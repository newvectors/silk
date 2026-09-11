package win32

import (
	"syscall"
	"testing"
	"unsafe"
)

// Make cross-compilation check the ABI too: these assertions fail at compile
// time, even when this test executable cannot run on the build host.
const expectedNativeInputSize = 16 + 3*unsafe.Sizeof(uintptr(0))
const expectedNativeInputOffset = unsafe.Sizeof(uintptr(0))

var (
	_ [unsafe.Sizeof(nativeInput{}) - expectedNativeInputSize]byte
	_ [expectedNativeInputSize - unsafe.Sizeof(nativeInput{})]byte
	_ [unsafe.Offsetof(nativeInput{}.Data) - expectedNativeInputOffset]byte
	_ [expectedNativeInputOffset - unsafe.Offsetof(nativeInput{}.Data)]byte
)

func TestNativeInputABI(t *testing.T) {
	var input nativeInput
	wantSize, wantOffset := uintptr(28), uintptr(4)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		wantSize, wantOffset = 40, 8
	}
	if size, offset := unsafe.Sizeof(input), unsafe.Offsetof(input.Data); size != wantSize || offset != wantOffset {
		t.Fatalf("INPUT size/union offset = %d/%d, want %d/%d", size, offset, wantSize, wantOffset)
	}
}

func TestMarshalInputUnions(t *testing.T) {
	mouse := MOUSEINPUT{Dx: -123, Dy: 456, MouseData: 5, DwFlags: 6, Time: 7, DwExtraInfo: ^uintptr(0)}
	keyboard := KEYBDINPUT{WVk: 65, WScan: 123, DwFlags: 4, Time: 5, DwExtraInfo: ^uintptr(0)}
	hardware := HARDWAREINPUT{UMsg: 456, WParamL: 7, WParamH: 8}
	inputs := marshalInputs([]INPUT{
		{Type: INPUT_MOUSE, Mi: mouse},
		{Type: INPUT_KEYBOARD, Ki: keyboard},
		{Type: INPUT_HARDWARE, Hi: hardware},
	})
	if len(inputs) != 3 || inputs[0].Type != INPUT_MOUSE || inputs[0].Data != mouse {
		t.Fatal("mouse INPUT failed to preserve data")
	}
	if inputs[1].Type != INPUT_KEYBOARD || *(*KEYBDINPUT)(unsafe.Pointer(&inputs[1].Data)) != keyboard {
		t.Fatal("keyboard INPUT failed to preserve data or full-width extra info")
	}
	if inputs[2].Type != INPUT_HARDWARE || *(*HARDWAREINPUT)(unsafe.Pointer(&inputs[2].Data)) != hardware {
		t.Fatal("hardware INPUT failed to preserve data")
	}
	if n := SendInput(nil); n != 0 {
		t.Fatalf("empty input = %d, want 0 without calling User32", n)
	}
}

func TestWindowLongPtrEntryPoints(t *testing.T) {
	for _, operation := range []string{"Get", "Set"} {
		want := operation + "WindowLongW"
		if unsafe.Sizeof(uintptr(0)) == 8 {
			want = operation + "WindowLongPtrW"
		}
		if got := windowLongPtrProcName(operation); got != want {
			t.Fatalf("%s pointer entry point = %q, want %q", operation, got, want)
		}
	}
	// Resolve the real exports, without opening windows or sending input.
	for _, proc := range []*syscall.LazyProc{procGetWindowLongPtr, procSetWindowLongPtr, procGetParent} {
		if err := proc.Find(); err != nil {
			t.Fatal(err)
		}
	}
}
