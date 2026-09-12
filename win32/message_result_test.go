package win32

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestGetMessageResultThreeStates(t *testing.T) {
	apiErr := syscall.Errno(1400)
	for _, tc := range []struct {
		name   string
		native uintptr
		last   error
		want   bool
		fail   bool
	}{
		{"message", 1, nil, true, false},
		{"message ignores stale last-error", 1, apiErr, true, false},
		{"quit", 0, nil, false, false},
		{"quit ignores stale last-error", 0, apiErr, false, false},
		{"error zero-extended DWORD", uintptr(0xffffffff), apiErr, false, true},
		{"error sign-extended native", ^uintptr(0), apiErr, false, true},
		{"error missing last-error", uintptr(0xffffffff), nil, false, true},
		{"error zero errno", uintptr(0xffffffff), syscall.Errno(0), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeGetMessageResult(tc.native, tc.last)
			if got != tc.want || (err != nil) != tc.fail {
				t.Fatalf("result=%v err=%v", got, err)
			}
			if tc.fail && tc.last == apiErr && !errors.Is(err, apiErr) {
				t.Fatalf("error code lost: %v", err)
			}
			if tc.fail && !strings.Contains(err.Error(), "GetMessageW") {
				t.Fatalf("API missing in error: %v", err)
			}
		})
	}
}
