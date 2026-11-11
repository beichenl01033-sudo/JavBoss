#import <AppKit/AppKit.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdbool.h>

// Read only the target player's bounds and its monitor work area. No screen
// pixels, Accessibility permissions, shell helpers or private APIs are used.
bool javbossDockWindowInfo(int pid, double *values) {
    @autoreleasepool {
        CFArrayRef raw = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
        if (!raw) return false;
        NSArray *windows = (__bridge NSArray *)raw;
        CGRect bounds = CGRectZero;
        bool found = false;
        for (NSDictionary *window in windows) {
            if ([window[(id)kCGWindowOwnerPID] intValue] != pid || [window[(id)kCGWindowLayer] intValue] != 0) continue;
            if (CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)window[(id)kCGWindowBounds], &bounds) && bounds.size.width > 0) {
                found = true;
                break;
            }
        }
        CFRelease(raw);
        if (!found) return false;
        double mainHeight = CGDisplayBounds(CGMainDisplayID()).size.height;
        NSScreen *selected = nil;
        double best = 0;
        for (NSScreen *screen in NSScreen.screens) {
            NSRect frame = screen.frame;
            CGRect area = CGRectMake(frame.origin.x, mainHeight - NSMaxY(frame), frame.size.width, frame.size.height);
            CGRect overlap = CGRectIntersection(area, bounds);
            double size = CGRectIsNull(overlap) ? 0 : overlap.size.width * overlap.size.height;
            if (size > best) { best = size; selected = screen; }
        }
        if (!selected) return false;
        NSRect work = selected.visibleFrame;
        values[0] = bounds.origin.x; values[1] = bounds.origin.y;
        values[2] = bounds.size.width; values[3] = bounds.size.height;
        values[4] = work.origin.x; values[5] = mainHeight - NSMaxY(work);
        values[6] = work.size.width; values[7] = work.size.height;
        values[8] = selected.backingScaleFactor;
        return true;
    }
}
