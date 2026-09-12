package gui

// The blocking Win32 modal pump is separated from native calls so wakeups,
// nested modals and WM_QUIT propagation can be checked without an HWND. next
// must block, not poll; a message already fetched is dispatched exactly once.
type modalPumpMessage struct {
	window         uintptr
	id             uint32
	wParam, lParam uintptr
}
type modalPumpOps struct {
	next     func() (modalPumpMessage, bool)
	dispatch func()
	service  func()
	repost   func(modalPumpMessage)
	quit     func(int)
}

func pumpBlockingModalMessages(owner uintptr, endID, quitID uint32, ops modalPumpOps) {
	// A nested modal may retrieve its parent's EndModal message. Do not close
	// the wrong dialog or discard that message: put it back when this loop ends.
	var deferredEnds []modalPumpMessage
	defer func() {
		for _, msg := range deferredEnds {
			ops.repost(msg)
		}
	}()
	for {
		msg, ok := ops.next()
		// GetMessage returns false for WM_QUIT, so inspect it BEFORE the bool.
		if msg.id == quitID {
			ops.quit(int(msg.wParam))
			return
		}
		if !ok {
			return
		}
		if msg.id == endID {
			if msg.window == owner {
				return
			}
			deferredEnds = append(deferredEnds, msg)
		} else {
			ops.dispatch()
		}
		ops.service()
	}
}
