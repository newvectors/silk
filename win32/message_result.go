package win32

import (
	"fmt"
	"syscall"
)

// Pure decoding of the signed 32-bit BOOL returned by GetMessageW. Keeping
// this independent of User32 lets non-Windows tests cover both ABI encodings
// of -1 without creating a native window or touching a message queue.
func decodeGetMessageResult(result uintptr, lastError error) (bool, error) {
	switch int32(result) {
	case -1:
		if lastError == nil || lastError == syscall.Errno(0) {
			return false, fmt.Errorf("win32: GetMessageW failed without an error code")
		}
		return false, fmt.Errorf("win32: GetMessageW: %w", lastError)
	case 0:
		return false, nil
	default:
		return true, nil
	}
}
