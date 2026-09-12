//go:build darwin && cgo

#import <Cocoa/Cocoa.h>

int silkApplicationActive(void) {
    if (![NSThread isMainThread]) {
        return -1;
    }
    return NSApp && [NSApp isActive] ? 1 : 0;
}

int silkBindPopupOwner(void *popupHandle, void *ownerHandle) {
    if (!popupHandle || !ownerHandle || ![NSThread isMainThread]) {
        return 0;
    }
    NSWindow *popup = (NSWindow *)popupHandle;
    NSWindow *owner = (NSWindow *)ownerHandle;
    // AppKit forbids a cycle in the attached-window tree.
    for (NSWindow *ancestor = owner; ancestor; ancestor = [ancestor parentWindow]) {
        if (ancestor == popup) {
            return 0;
        }
    }

    // GLFW's Floating flag uses NSFloatingWindowLevel, which sits above all
    // normal windows, not just this owner. Relative child ordering supplies
    // the required stacking without a globally floating palette.
    [popup setLevel:NSNormalWindowLevel];
    [popup setHidesOnDeactivate:YES];
    NSWindow *previous = [popup parentWindow];
    if (previous != owner) {
        if (previous) {
            [previous removeChildWindow:popup];
        }
        [owner addChildWindow:popup ordered:NSWindowAbove];
    }
    return [popup parentWindow] == owner;
}

void silkUnbindPopupOwner(void *popupHandle) {
    if (!popupHandle || ![NSThread isMainThread]) {
        return;
    }
    NSWindow *popup = (NSWindow *)popupHandle;
    NSWindow *owner = [popup parentWindow];
    if (owner) {
        [owner removeChildWindow:popup];
    }
}
