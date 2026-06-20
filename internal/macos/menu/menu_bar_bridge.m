#import <Cocoa/Cocoa.h>
#import <ServiceManagement/ServiceManagement.h>
#import <math.h>
#import <stdlib.h>
#import <string.h>

#import "menu_bar_bridge.h"

#ifndef __has_feature
#define __has_feature(feature) 0
#endif
#if !__has_feature(objc_arc)
#error "OpenTamer Objective-C menu bridge must be compiled with ARC; ensure cgo passes -fobjc-arc."
#endif

extern void opentamer_menu_command(const char *command);

@protocol OpenTamerPanelActionHandling
- (void)performPanelAction:(NSDictionary *)action event:(NSEvent *)event sourceView:(NSView *)view;
- (void)performPanelQuit;
@end

@interface OpenTamerPanelActionButton : NSButton
@property(nonatomic, strong) NSDictionary *panelAction;
@end

@implementation OpenTamerPanelActionButton

- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self == nil) {
        return nil;
    }
    self.title = @"";
    self.bordered = NO;
    self.enabled = YES;
    self.buttonType = NSButtonTypeMomentaryChange;
    self.focusRingType = NSFocusRingTypeNone;
    return self;
}

- (void)drawRect:(NSRect)dirtyRect {
}

- (NSView *)hitTest:(NSPoint)point {
    return NSPointInRect(point, self.bounds) ? self : nil;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (void)mouseDown:(NSEvent *)event {
    [NSApp sendAction:self.action to:self.target from:self];
}

@end

@interface OpenTamerCommandMenuItemView : NSView
@property(nonatomic, weak) NSMenuItem *menuItem;
@property(nonatomic, copy) NSString *title;
@property(nonatomic, assign) BOOL checked;
@property(nonatomic, assign, getter=isEnabled) BOOL enabled;
@property(nonatomic, weak) id target;
@property(nonatomic, assign) SEL action;
@end

@implementation OpenTamerCommandMenuItemView

- (instancetype)initWithTitle:(NSString *)title {
    self = [super initWithFrame:NSMakeRect(0, 0, 240, 22)];
    if (self == nil) {
        return nil;
    }
    self.title = title ?: @"";
    self.enabled = YES;
    self.autoresizingMask = NSViewWidthSizable;
    return self;
}

- (BOOL)isFlipped {
    return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (NSView *)hitTest:(NSPoint)point {
    return NSPointInRect(point, self.bounds) ? self : nil;
}

- (void)mouseDown:(NSEvent *)event {
    if (!self.enabled || self.action == nil || self.target == nil) {
        return;
    }
    [NSApp sendAction:self.action to:self.target from:self];
}

- (void)drawRect:(NSRect)dirtyRect {
    NSColor *textColor = self.enabled ? [NSColor labelColor] : [NSColor disabledControlTextColor];
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont menuFontOfSize:0],
        NSForegroundColorAttributeName: textColor
    };
    if (self.checked) {
        [@"\u2713" drawInRect:NSMakeRect(7, 3, 14, 16) withAttributes:attrs];
    }
    [self.title drawInRect:NSMakeRect(26, 3, NSWidth(self.bounds) - 34, 16) withAttributes:attrs];
}

@end

static BOOL OpenTamerCommandShouldKeepMenuOpen(NSString *command) {
    return [command hasPrefix:@"pref-"] ||
        [command hasPrefix:@"graph-window|"];
}

@interface OpenTamerPersistentMenu : NSMenu
@end

@implementation OpenTamerPersistentMenu

- (instancetype)initWithTitle:(NSString *)title {
    self = [super initWithTitle:title];
    if (self == nil) {
        return nil;
    }
    self.autoenablesItems = NO;
    return self;
}

- (void)performActionForItemAtIndex:(NSInteger)index {
    NSMenuItem *item = [self itemAtIndex:index];
    NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
    if (OpenTamerCommandShouldKeepMenuOpen(command) && item.action != nil && item.target != nil) {
        [NSApp sendAction:item.action to:item.target from:item];
        return;
    }
    [super performActionForItemAtIndex:index];
}

@end

static NSString *OpenTamerFormattedStatusPercent(double value) {
    if (value > 0 && value < 0.01) {
        return @"<0.01%";
    }
    if (value > 0 && value < 1) {
        return [NSString stringWithFormat:@"%.2f%%", value];
    }
    if (value > 0 && value < 10) {
        return [NSString stringWithFormat:@"%.1f%%", value];
    }
    return [NSString stringWithFormat:@"%.0f%%", value];
}

static NSString *OpenTamerStringFromValue(id value, NSString *fallback) {
    return [value isKindOfClass:NSString.class] ? value : fallback;
}

static double OpenTamerDoubleFromRow(NSDictionary *row, NSString *key, double fallback) {
    id value = row[key];
    if (value == nil || value == [NSNull null] || ![value respondsToSelector:@selector(doubleValue)]) {
        return fallback;
    }
    return [value doubleValue];
}

static NSString *OpenTamerCPUDisplayMode(NSString *mode) {
    return [mode isEqualToString:@"system_normalized"] ? @"system_normalized" : @"per_core_process";
}

static BOOL OpenTamerUsesSystemNormalizedCPU(NSString *mode) {
    return [OpenTamerCPUDisplayMode(mode) isEqualToString:@"system_normalized"];
}

static double OpenTamerSystemCPUFromRow(NSDictionary *row) {
    return OpenTamerDoubleFromRow(row, @"systemCPUPercent", OpenTamerDoubleFromRow(row, @"cpuPercent", 0));
}

static double OpenTamerDisplayCPUFromRow(NSDictionary *row, NSString *mode) {
    if (OpenTamerUsesSystemNormalizedCPU(mode)) {
        return OpenTamerSystemCPUFromRow(row);
    }
    return OpenTamerDoubleFromRow(row, @"cpuPercent", OpenTamerSystemCPUFromRow(row));
}

static double OpenTamerAverageDisplayCPUFromRow(NSDictionary *row, NSString *mode) {
    if (OpenTamerUsesSystemNormalizedCPU(mode)) {
        return OpenTamerDoubleFromRow(row, @"averageSystemCPUPercent", OpenTamerSystemCPUFromRow(row));
    }
    return OpenTamerDoubleFromRow(row, @"averageCPUPercent", OpenTamerDoubleFromRow(row, @"cpuPercent", 0));
}

static NSColor *OpenTamerGraphColor(NSUInteger index) {
    switch (index % 10) {
        case 0:
            return [NSColor colorWithCalibratedRed:0.10 green:0.55 blue:0.28 alpha:1.0];
        case 1:
            return [NSColor colorWithCalibratedRed:0.10 green:0.38 blue:0.90 alpha:1.0];
        case 2:
            return [NSColor colorWithCalibratedRed:0.88 green:0.31 blue:0.20 alpha:1.0];
        case 3:
            return [NSColor colorWithCalibratedRed:0.58 green:0.28 blue:0.82 alpha:1.0];
        case 4:
            return [NSColor colorWithCalibratedRed:0.93 green:0.62 blue:0.12 alpha:1.0];
        case 5:
            return [NSColor colorWithCalibratedRed:0.03 green:0.60 blue:0.62 alpha:1.0];
        case 6:
            return [NSColor colorWithCalibratedRed:0.78 green:0.20 blue:0.48 alpha:1.0];
        case 7:
            return [NSColor colorWithCalibratedRed:0.36 green:0.48 blue:0.10 alpha:1.0];
        case 8:
            return [NSColor colorWithCalibratedRed:0.20 green:0.48 blue:0.78 alpha:1.0];
        default:
            return [NSColor colorWithCalibratedRed:0.62 green:0.36 blue:0.12 alpha:1.0];
    }
}

static const CGFloat OpenTamerStatusItemLengthWithIcon = 78.0;
static const CGFloat OpenTamerStatusItemLengthTextOnly = 52.0;

@interface OpenTamerPanelView : NSView
@property(nonatomic, assign) id<OpenTamerPanelActionHandling> controller;
@property(nonatomic, strong) NSMutableArray *actions;
@property(nonatomic, assign) BOOL handledMouseDown;
- (void)addActionNamed:(NSString *)name frame:(NSRect)frame row:(NSDictionary *)row kind:(NSString *)kind title:(NSString *)title;
- (void)installActionButtonsWithTarget:(id)target action:(SEL)selector;
@end

@implementation OpenTamerPanelView

- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self == nil) {
        return nil;
    }
    self.appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
    self.actions = [[NSMutableArray alloc] init];
    return self;
}

- (BOOL)isFlipped {
    return YES;
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
    return YES;
}

- (BOOL)acceptsFirstResponder {
    return YES;
}

- (NSView *)hitTest:(NSPoint)point {
    if (!NSPointInRect(point, self.bounds)) {
        return nil;
    }
    NSView *hit = [super hitTest:point];
    return hit ?: self;
}

- (void)drawRect:(NSRect)dirtyRect {
    [[NSColor colorWithCalibratedWhite:0.985 alpha:1.0] setFill];
    NSRectFill(self.bounds);

    for (NSDictionary *action in self.actions) {
        NSString *name = [action[@"name"] isKindOfClass:NSString.class] ? action[@"name"] : @"";
        if ([name isEqualToString:@"toggle"]) {
            [self drawToggleAction:action];
        } else if ([name isEqualToString:@"refresh"] || [name isEqualToString:@"preferences"] || [name isEqualToString:@"all-processes"]) {
            [self drawButtonAction:action];
        } else if ([name isEqualToString:@"process"] || [name isEqualToString:@"managed"]) {
            [self drawProcessAction:action];
        }
    }
}

- (void)addActionNamed:(NSString *)name frame:(NSRect)frame row:(NSDictionary *)row kind:(NSString *)kind title:(NSString *)title {
    NSMutableDictionary *action = [@{
        @"name": name ?: @"",
        @"frame": [NSValue valueWithRect:frame]
    } mutableCopy];
    if (row != nil) {
        action[@"row"] = row;
    }
    if (kind != nil) {
        action[@"kind"] = kind;
    }
    if (title != nil) {
        action[@"title"] = title;
    }
    [self.actions addObject:action];
}

- (void)installActionButtonsWithTarget:(id)target action:(SEL)selector {
    for (NSDictionary *action in self.actions) {
        NSRect frame = [action[@"frame"] rectValue];
        OpenTamerPanelActionButton *button = [[OpenTamerPanelActionButton alloc] initWithFrame:frame];
        button.panelAction = action;
        button.target = target;
        button.action = selector;
        [self addSubview:button positioned:NSWindowAbove relativeTo:nil];
    }
}

- (NSDictionary *)actionAtPoint:(NSPoint)point {
    for (NSDictionary *action in [self.actions reverseObjectEnumerator]) {
        NSRect frame = [action[@"frame"] rectValue];
        if (!NSPointInRect(point, frame)) {
            continue;
        }
        return action;
    }
    return nil;
}

- (void)performActionForEvent:(NSEvent *)event {
    NSPoint point = [self convertPoint:event.locationInWindow fromView:nil];
    NSDictionary *action = [self actionAtPoint:point];
    if (action == nil) {
        return;
    }
    [self.controller performPanelAction:action event:event sourceView:self];
}

- (void)mouseDown:(NSEvent *)event {
    self.handledMouseDown = YES;
    [self performActionForEvent:event];
}

- (void)mouseUp:(NSEvent *)event {
    // Modal NSMenu tracking can deliver a stale mouse-up after a button opened
    // a submenu. Do not treat mouse-up as a fresh panel action.
    self.handledMouseDown = NO;
}

- (BOOL)isCommandQEvent:(NSEvent *)event {
    if (event.type != NSEventTypeKeyDown) {
        return NO;
    }
    NSEventModifierFlags flags = event.modifierFlags & NSEventModifierFlagDeviceIndependentFlagsMask;
    if (flags != NSEventModifierFlagCommand) {
        return NO;
    }
    NSString *characters = event.charactersIgnoringModifiers.lowercaseString;
    return [characters isEqualToString:@"q"];
}

- (BOOL)performKeyEquivalent:(NSEvent *)event {
    if ([self isCommandQEvent:event]) {
        [self.controller performPanelQuit];
        return YES;
    }
    return [super performKeyEquivalent:event];
}

- (void)keyDown:(NSEvent *)event {
    if ([self isCommandQEvent:event]) {
        [self.controller performPanelQuit];
        return;
    }
    [super keyDown:event];
}

- (void)drawToggleAction:(NSDictionary *)action {
    NSRect bounds = NSInsetRect([action[@"frame"] rectValue], 1, 3);
    BOOL toggleOn = [action[@"toggleOn"] boolValue];
    NSColor *track = toggleOn
        ? [NSColor colorWithCalibratedRed:0.11 green:0.68 blue:0.32 alpha:1.0]
        : [NSColor colorWithCalibratedWhite:0.70 alpha:1.0];
    [track setFill];
    [[NSBezierPath bezierPathWithRoundedRect:bounds xRadius:NSHeight(bounds) / 2 yRadius:NSHeight(bounds) / 2] fill];

    CGFloat knobSize = NSHeight(bounds) - 4;
    CGFloat knobX = toggleOn ? NSMaxX(bounds) - knobSize - 2 : NSMinX(bounds) + 2;
    NSRect knob = NSMakeRect(knobX, NSMinY(bounds) + 2, knobSize, knobSize);
    [[NSColor whiteColor] setFill];
    [[NSBezierPath bezierPathWithOvalInRect:knob] fill];
}

- (void)drawButtonAction:(NSDictionary *)action {
    NSRect bounds = [action[@"frame"] rectValue];
    NSString *title = [action[@"title"] isKindOfClass:NSString.class] ? action[@"title"] : @"";
    [[NSColor colorWithCalibratedWhite:0.92 alpha:1.0] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:bounds xRadius:6 yRadius:6] fill];

    NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
    style.alignment = [action[@"name"] isEqualToString:@"all-processes"] ? NSTextAlignmentLeft : NSTextAlignmentCenter;
    style.lineBreakMode = NSLineBreakByTruncatingTail;
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:12 weight:NSFontWeightMedium],
        NSForegroundColorAttributeName: [NSColor labelColor],
        NSParagraphStyleAttributeName: style
    };
    NSRect textRect = NSInsetRect(bounds, [action[@"name"] isEqualToString:@"all-processes"] ? 12 : 4, 5);
    [title drawInRect:textRect withAttributes:attrs];
}

- (void)drawProcessAction:(NSDictionary *)action {
    NSRect bounds = NSInsetRect([action[@"frame"] rectValue], 2, 2);
    [[NSColor colorWithCalibratedWhite:1.0 alpha:0.72] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:bounds xRadius:6 yRadius:6] fill];

    NSDictionary *row = [action[@"row"] isKindOfClass:NSDictionary.class] ? action[@"row"] : @{};
    NSString *kind = [action[@"kind"] isKindOfClass:NSString.class] ? action[@"kind"] : @"";
    NSString *name = OpenTamerStringFromValue(row[@"name"], @"Unknown");
    NSString *displayMode = OpenTamerCPUDisplayMode(OpenTamerStringFromValue(action[@"cpuDisplayMode"], @"per_core_process"));
    double displayCPU = OpenTamerDisplayCPUFromRow(row, displayMode);
    double averageCPU = OpenTamerAverageDisplayCPUFromRow(row, displayMode);
    NSString *right = [NSString stringWithFormat:@"%@  %@",
                       OpenTamerFormattedStatusPercent(displayCPU),
                       OpenTamerFormattedStatusPercent(averageCPU)];
    if ([kind isEqualToString:@"managed"]) {
        NSString *rule = OpenTamerStringFromValue(row[@"ruleLabel"], @"Rule");
        right = [NSString stringWithFormat:@"%@  %@", OpenTamerFormattedStatusPercent(displayCPU), rule];
    }

    NSMutableParagraphStyle *nameStyle = [[NSMutableParagraphStyle alloc] init];
    nameStyle.lineBreakMode = NSLineBreakByTruncatingTail;
    NSMutableParagraphStyle *valueStyle = [[NSMutableParagraphStyle alloc] init];
    valueStyle.alignment = NSTextAlignmentRight;
    valueStyle.lineBreakMode = NSLineBreakByTruncatingTail;

    NSDictionary *nameAttrs = @{
        NSFontAttributeName: [NSFont systemFontOfSize:12 weight:NSFontWeightMedium],
        NSForegroundColorAttributeName: [NSColor labelColor],
        NSParagraphStyleAttributeName: nameStyle
    };
    NSDictionary *valueAttrs = @{
        NSFontAttributeName: [NSFont monospacedDigitSystemFontOfSize:11 weight:NSFontWeightRegular],
        NSForegroundColorAttributeName: [NSColor labelColor],
        NSParagraphStyleAttributeName: valueStyle
    };

    CGFloat valueWidth = [kind isEqualToString:@"managed"] ? 162 : 116;
    NSRect valueRect = NSMakeRect(NSMaxX(bounds) - valueWidth - 8, NSMinY(bounds) + 6, valueWidth, 16);
    CGFloat nameX = NSMinX(bounds) + 8;
    id graphColorIndex = action[@"graphColorIndex"];
    if (graphColorIndex != nil && graphColorIndex != [NSNull null] && [graphColorIndex respondsToSelector:@selector(unsignedIntegerValue)]) {
        NSRect dotRect = NSMakeRect(nameX, NSMinY(bounds) + 8, 8, 8);
        NSColor *dotColor = OpenTamerGraphColor([graphColorIndex unsignedIntegerValue]);
        NSBezierPath *dotPath = [NSBezierPath bezierPathWithOvalInRect:dotRect];
        BOOL graphLineHidden = [action[@"graphLineHidden"] boolValue];
        if (graphLineHidden) {
            [[dotColor colorWithAlphaComponent:0.16] setFill];
            [dotPath fill];
            [dotColor setStroke];
            dotPath.lineWidth = 1.1;
            [dotPath stroke];

            NSBezierPath *slash = [NSBezierPath bezierPath];
            slash.lineWidth = 1.3;
            [slash moveToPoint:NSMakePoint(NSMinX(dotRect) + 2, NSMaxY(dotRect) - 2)];
            [slash lineToPoint:NSMakePoint(NSMaxX(dotRect) - 2, NSMinY(dotRect) + 2)];
            [slash stroke];
        } else {
            [dotColor setFill];
            [dotPath fill];
        }
        nameX += 16;
    }
    NSRect nameRect = NSMakeRect(nameX, NSMinY(bounds) + 6, MAX((CGFloat)0, NSMinX(valueRect) - nameX - 2), 16);
    [name drawInRect:nameRect withAttributes:nameAttrs];
    [right drawInRect:valueRect withAttributes:valueAttrs];
}

@end

@interface OpenTamerCPUGraphView : NSView
@property(nonatomic, copy) NSArray *lines;
@property(nonatomic, copy) NSSet *hiddenAppKeys;
@property(nonatomic, assign) double currentCPU;
@property(nonatomic, assign) double windowStartUnix;
@property(nonatomic, assign) double windowEndUnix;
@end

@implementation OpenTamerCPUGraphView

- (BOOL)isFlipped {
    return YES;
}

- (BOOL)lineIsHidden:(NSDictionary *)lineObject {
    NSString *appKey = OpenTamerStringFromValue(lineObject[@"appKey"], @"");
    return appKey.length > 0 && [self.hiddenAppKeys containsObject:appKey];
}

- (double)maxGraphValue {
    double maximum = 0;
    BOOL hasLinePoint = NO;
    for (id lineObject in self.lines) {
        if (![lineObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        if ([self lineIsHidden:lineObject]) {
            continue;
        }
        NSArray *points = [lineObject[@"points"] isKindOfClass:NSArray.class] ? lineObject[@"points"] : @[];
        for (id pointObject in points) {
            if (![pointObject isKindOfClass:NSDictionary.class]) {
                continue;
            }
            double value = OpenTamerDoubleFromRow(pointObject, @"appCPU", 0);
            hasLinePoint = YES;
            if (value > maximum) {
                maximum = value;
            }
        }
    }
    if (!hasLinePoint) {
        maximum = self.currentCPU;
    }
    if (maximum < 1) {
        return 1;
    }
    if (maximum < 10) {
        return 10;
    }
    return maximum * 1.15;
}

- (CGFloat)xPositionForPoint:(NSDictionary *)pointObject index:(NSUInteger)index count:(NSUInteger)count plot:(NSRect)plot {
    double atUnix = OpenTamerDoubleFromRow(pointObject, @"atUnix", 0);
    if (self.windowEndUnix > self.windowStartUnix && atUnix > 0) {
        double ratio = (atUnix - self.windowStartUnix) / (self.windowEndUnix - self.windowStartUnix);
        if (ratio < 0) {
            ratio = 0;
        }
        if (ratio > 1) {
            ratio = 1;
        }
        return NSMinX(plot) + (NSWidth(plot) * (CGFloat)ratio);
    }
    if (count <= 1) {
        return NSMinX(plot);
    }
    return NSMinX(plot) + (NSWidth(plot) * ((CGFloat)index / (CGFloat)(count - 1)));
}

- (void)drawYAxisForPlot:(NSRect)plot maximum:(double)maximum {
    NSMutableParagraphStyle *labelStyle = [[NSMutableParagraphStyle alloc] init];
    labelStyle.alignment = NSTextAlignmentLeft;
    labelStyle.lineBreakMode = NSLineBreakByClipping;
    NSDictionary *labelAttrs = @{
        NSFontAttributeName: [NSFont monospacedDigitSystemFontOfSize:9 weight:NSFontWeightRegular],
        NSForegroundColorAttributeName: [NSColor secondaryLabelColor],
        NSParagraphStyleAttributeName: labelStyle
    };

    [[NSColor colorWithCalibratedWhite:0.62 alpha:0.75] setStroke];
    NSBezierPath *axis = [NSBezierPath bezierPath];
    axis.lineWidth = 0.7;
    [axis moveToPoint:NSMakePoint(NSMaxX(plot), NSMinY(plot))];
    [axis lineToPoint:NSMakePoint(NSMaxX(plot), NSMaxY(plot))];
    [axis stroke];

    for (NSInteger i = 0; i < 4; i++) {
        CGFloat y = NSMinY(plot) + (NSHeight(plot) / 3.0) * i;
        double value = maximum * (1.0 - ((double)i / 3.0));

        NSBezierPath *tick = [NSBezierPath bezierPath];
        tick.lineWidth = 0.7;
        [tick moveToPoint:NSMakePoint(NSMaxX(plot), y)];
        [tick lineToPoint:NSMakePoint(NSMaxX(plot) + 4, y)];
        [tick stroke];

        NSString *label = OpenTamerFormattedStatusPercent(value);
        NSRect labelRect = NSMakeRect(NSMaxX(plot) + 7, y - 7, 34, 14);
        [label drawInRect:labelRect withAttributes:labelAttrs];
    }
}

- (void)drawRect:(NSRect)dirtyRect {
    NSRect bounds = NSInsetRect(self.bounds, 1, 1);
    [[NSColor colorWithCalibratedWhite:1.0 alpha:1.0] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:bounds xRadius:8 yRadius:8] fill];

    NSRect plot = NSMakeRect(NSMinX(bounds) + 12, NSMinY(bounds) + 14, NSWidth(bounds) - 58, NSHeight(bounds) - 28);
    [[NSColor colorWithCalibratedWhite:0.80 alpha:0.55] setStroke];
    for (NSInteger i = 0; i < 4; i++) {
        CGFloat y = NSMinY(plot) + (NSHeight(plot) / 3.0) * i;
        NSBezierPath *grid = [NSBezierPath bezierPath];
        grid.lineWidth = 0.5;
        [grid moveToPoint:NSMakePoint(NSMinX(plot), y)];
        [grid lineToPoint:NSMakePoint(NSMaxX(plot), y)];
        [grid stroke];
    }

    double maximum = [self maxGraphValue];
    [self drawYAxisForPlot:plot maximum:maximum];
    NSUInteger lineIndex = 0;
    NSUInteger graphLineCount = 0;
    NSUInteger hiddenLineCount = 0;
    NSUInteger drawnLineCount = 0;
    for (id lineObject in self.lines) {
        if (![lineObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        graphLineCount++;
        NSDictionary *line = lineObject;
        if ([self lineIsHidden:line]) {
            hiddenLineCount++;
            lineIndex++;
            continue;
        }
        NSArray *points = [lineObject[@"points"] isKindOfClass:NSArray.class] ? lineObject[@"points"] : @[];
        if (points.count == 0) {
            lineIndex++;
            continue;
        }

        NSBezierPath *path = [NSBezierPath bezierPath];
        path.lineWidth = 2.0;
        for (NSUInteger i = 0; i < points.count; i++) {
            id pointObject = points[i];
            if (![pointObject isKindOfClass:NSDictionary.class]) {
                continue;
            }
            double value = OpenTamerDoubleFromRow(pointObject, @"appCPU", 0);
            CGFloat x = [self xPositionForPoint:pointObject index:i count:points.count plot:plot];
            CGFloat y = NSMaxY(plot) - (NSHeight(plot) * (CGFloat)(value / maximum));
            if (path.elementCount == 0) {
                [path moveToPoint:NSMakePoint(x, y)];
            } else {
                [path lineToPoint:NSMakePoint(x, y)];
            }
        }
        [OpenTamerGraphColor(lineIndex) setStroke];
        [path stroke];
        drawnLineCount++;
        lineIndex++;
    }

    if (graphLineCount == 0 || drawnLineCount == 0) {
        NSDictionary *attrs = @{
            NSFontAttributeName: [NSFont systemFontOfSize:12],
            NSForegroundColorAttributeName: [NSColor secondaryLabelColor]
        };
        NSString *message = graphLineCount > 0 && hiddenLineCount == graphLineCount
            ? @"All graph lines hidden"
            : @"Collecting CPU history";
        [message drawInRect:NSInsetRect(bounds, 16, 46) withAttributes:attrs];
    }
}

@end

@interface OpenTamerMenuBarController : NSObject <NSMenuDelegate, NSPopoverDelegate>
@property(nonatomic, strong) NSStatusItem *statusItem;
@property(nonatomic, strong) NSDictionary *state;
@property(nonatomic, strong) NSMenu *mainMenu;
@property(nonatomic, strong) NSPopover *primaryPopover;
@property(nonatomic, strong) NSMutableDictionary *trackedStatusItems;
@property(nonatomic, strong) NSMutableSet *hiddenGraphAppKeys;
@property(nonatomic, strong) NSImage *statusIcon;
@property(nonatomic, assign) BOOL menuVisible;
@property(nonatomic, assign) BOOL needsMenuRebuild;
@end

@implementation OpenTamerMenuBarController

- (instancetype)initWithState:(NSDictionary *)state {
    self = [super init];
    if (self == nil) {
        return nil;
    }
    _state = state ?: @{};
    _trackedStatusItems = [NSMutableDictionary dictionary];
    _hiddenGraphAppKeys = [NSMutableSet set];
    return self;
}

- (void)install {
    self.statusItem = [NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength];
    self.statusItem.menu = nil;
    [self updateStatusTitle];
    [self updateTrackedStatusItems];
    [self rebuildMenu];
}

- (void)updateWithState:(NSDictionary *)state {
    self.state = state ?: @{};
    [self pruneHiddenGraphAppKeys];
    [self updateStatusTitle];
    [self updateTrackedStatusItems];
    if (self.primaryPopover.isShown) {
        [self refreshPrimaryPopoverContent];
        return;
    }
    if (self.menuVisible) {
        self.needsMenuRebuild = YES;
    } else {
        [self rebuildMenu];
    }
}

- (BOOL)managementEnabled {
    id enabled = self.state[@"enabled"];
    return enabled == nil ? YES : [enabled boolValue];
}

- (BOOL)showMenuBarIcon {
    id enabled = self.state[@"showMenuBarIcon"];
    return enabled == nil ? YES : [enabled boolValue];
}

- (double)totalCPU {
    return [self.state[@"totalCPU"] doubleValue];
}

- (NSString *)alertLevel {
    id value = self.state[@"alertLevel"];
    return [value isKindOfClass:NSString.class] ? value : @"normal";
}

- (NSString *)statusMessage {
    id value = self.state[@"statusMessage"];
    return [value isKindOfClass:NSString.class] ? value : @"";
}

- (NSArray *)topProcesses {
    id value = self.state[@"topProcesses"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (NSArray *)trackedApps {
    id value = self.state[@"trackedApps"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (NSArray *)menuBarApps {
    id value = self.state[@"menuBarApps"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (NSArray *)allProcesses {
    id value = self.state[@"allProcesses"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (NSArray *)managedApps {
    id value = self.state[@"managedApps"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (NSDictionary *)cpuGraph {
    id value = self.state[@"cpuGraph"];
    return [value isKindOfClass:NSDictionary.class] ? value : @{};
}

- (NSDictionary *)preferences {
    id value = self.state[@"preferences"];
    return [value isKindOfClass:NSDictionary.class] ? value : @{};
}

- (NSArray *)cpuGraphLines {
    id value = [self cpuGraph][@"lines"];
    return [value isKindOfClass:NSArray.class] ? value : @[];
}

- (double)cpuGraphCurrentCPU {
    id pointsObject = [self cpuGraph][@"points"];
    if (![pointsObject isKindOfClass:NSArray.class]) {
        return [self totalCPU];
    }
    NSArray *points = pointsObject;
    NSDictionary *point = [points.lastObject isKindOfClass:NSDictionary.class] ? points.lastObject : nil;
    if (point == nil) {
        return [self totalCPU];
    }
    return OpenTamerDoubleFromRow(point, @"totalCPU", [self totalCPU]);
}

- (BOOL)boolPreference:(NSString *)key fallback:(BOOL)fallback {
    id value = [self preferences][key];
    if (value == nil || value == [NSNull null] || ![value respondsToSelector:@selector(boolValue)]) {
        return fallback;
    }
    return [value boolValue];
}

- (BOOL)hasPreference:(NSString *)key {
    id value = [self preferences][key];
    return value != nil && value != [NSNull null];
}

- (double)durationPreferenceSeconds:(NSString *)key fallback:(double)fallback {
    id value = [self preferences][key];
    if (value == nil || value == [NSNull null] || ![value respondsToSelector:@selector(doubleValue)]) {
        return fallback;
    }
    return [value doubleValue] / 1000000000.0;
}

- (double)floatPreference:(NSString *)key fallback:(double)fallback {
    id value = [self preferences][key];
    if (value == nil || value == [NSNull null] || ![value respondsToSelector:@selector(doubleValue)]) {
        return fallback;
    }
    return [value doubleValue];
}

- (NSString *)stringPreference:(NSString *)key fallback:(NSString *)fallback {
    id value = [self preferences][key];
    return [value isKindOfClass:NSString.class] ? value : fallback;
}

- (NSSet *)currentGraphAppKeys {
    NSMutableSet *keys = [NSMutableSet set];
    for (id lineObject in [self cpuGraphLines]) {
        if (![lineObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        NSString *appKey = OpenTamerStringFromValue(lineObject[@"appKey"], @"");
        if (appKey.length > 0) {
            [keys addObject:appKey];
        }
    }
    return keys;
}

- (void)pruneHiddenGraphAppKeys {
    if (self.hiddenGraphAppKeys.count == 0) {
        return;
    }
    [self.hiddenGraphAppKeys intersectSet:[self currentGraphAppKeys]];
}

- (BOOL)isGraphLineHiddenForAppKey:(NSString *)appKey {
    return appKey.length > 0 && [self.hiddenGraphAppKeys containsObject:appKey];
}

- (void)toggleGraphLineForAppKey:(NSString *)appKey {
    if (appKey.length == 0) {
        return;
    }
    if ([self.hiddenGraphAppKeys containsObject:appKey]) {
        [self.hiddenGraphAppKeys removeObject:appKey];
    } else {
        [self.hiddenGraphAppKeys addObject:appKey];
    }
}

- (NSString *)formattedPercent:(double)value {
    return [NSString stringWithFormat:@"%.0f%%", value];
}

- (NSString *)formattedStatusPercent:(double)value {
    return OpenTamerFormattedStatusPercent(value);
}

- (double)doubleFromRow:(NSDictionary *)row key:(NSString *)key fallback:(double)fallback {
    id value = row[key];
    if (value == nil || value == [NSNull null] || ![value respondsToSelector:@selector(doubleValue)]) {
        return fallback;
    }
    return [value doubleValue];
}

- (double)systemCPUPercentForRow:(NSDictionary *)row {
    return OpenTamerSystemCPUFromRow(row);
}

- (NSString *)cpuDisplayMode {
    return OpenTamerCPUDisplayMode([self stringPreference:@"cpuDisplayMode" fallback:@"per_core_process"]);
}

- (NSString *)cpuDisplayLabel {
    return OpenTamerUsesSystemNormalizedCPU([self cpuDisplayMode]) ? @"System CPU" : @"Process CPU";
}

- (NSString *)topProcessesCPULabel {
    return OpenTamerUsesSystemNormalizedCPU([self cpuDisplayMode]) ? @"system CPU" : @"process CPU";
}

- (double)displayCPUPercentForRow:(NSDictionary *)row {
    return OpenTamerDisplayCPUFromRow(row, [self cpuDisplayMode]);
}

- (NSString *)stringFromValue:(id)value fallback:(NSString *)fallback {
    return [value isKindOfClass:NSString.class] ? value : fallback;
}

- (NSString *)appKeyFromRow:(NSDictionary *)row {
    return [self stringFromValue:row[@"appKey"] fallback:@""];
}

- (NSString *)cpuTitleForRow:(NSDictionary *)row {
    return [self formattedStatusPercent:[self displayCPUPercentForRow:row]];
}

- (NSString *)topProcessesSortMode {
    NSString *mode = [self stringPreference:@"topProcessesSort" fallback:@"current"];
    return [mode isEqualToString:@"average"] ? @"average" : @"current";
}

- (NSString *)topProcessesHeaderTitle {
    if ([[self topProcessesSortMode] isEqualToString:@"average"]) {
        return [NSString stringWithFormat:@"Top Processes (60s avg %@)", [self topProcessesCPULabel]];
    }
    return [NSString stringWithFormat:@"Top Processes (current %@)", [self topProcessesCPULabel]];
}

- (void)addCPUSummaryForRow:(NSDictionary *)row toMenu:(NSMenu *)menu {
    [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"%@: %@", [self cpuDisplayLabel], [self formattedStatusPercent:[self displayCPUPercentForRow:row]]]]];
}

- (NSString *)shortStatusName:(NSString *)name {
    NSString *trimmed = [name stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    if (trimmed.length == 0) {
        return @"App";
    }
    if (trimmed.length <= 16) {
        return trimmed;
    }
    return [NSString stringWithFormat:@"%@...", [trimmed substringToIndex:13]];
}

- (NSAttributedString *)statusTitleWithText:(NSString *)text {
    NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
    style.alignment = NSTextAlignmentCenter;
    NSDictionary *attrs = @{
        NSFontAttributeName: [NSFont monospacedDigitSystemFontOfSize:13 weight:NSFontWeightRegular],
        NSForegroundColorAttributeName: [NSColor labelColor],
        NSParagraphStyleAttributeName: style
    };
    return [[NSAttributedString alloc] initWithString:text attributes:attrs];
}

- (NSImage *)openTamerStatusIcon {
    if (self.statusIcon != nil) {
        return self.statusIcon;
    }

    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    [image lockFocus];
    [[NSColor blackColor] setStroke];

    NSBezierPath *loop = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(3.25, 3.25, 11.5, 11.5)];
    loop.lineWidth = 1.8;
    [loop stroke];

    NSBezierPath *tail = [NSBezierPath bezierPath];
    tail.lineWidth = 1.8;
    [tail moveToPoint:NSMakePoint(12.5, 5.2)];
    [tail curveToPoint:NSMakePoint(16, 2.8)
         controlPoint1:NSMakePoint(14.0, 4.7)
         controlPoint2:NSMakePoint(15.1, 3.7)];
    [tail stroke];

    NSArray *bars = @[
        [NSValue valueWithRect:NSMakeRect(6.0, 7.0, 1.5, 4.0)],
        [NSValue valueWithRect:NSMakeRect(8.4, 5.8, 1.5, 5.2)],
        [NSValue valueWithRect:NSMakeRect(10.8, 8.1, 1.5, 2.9)]
    ];
    [[NSColor blackColor] setFill];
    for (NSValue *value in bars) {
        NSBezierPath *bar = [NSBezierPath bezierPathWithRoundedRect:value.rectValue xRadius:0.7 yRadius:0.7];
        [bar fill];
    }

    [image unlockFocus];
    image.template = YES;
    self.statusIcon = image;
    return image;
}

- (void)updateStatusTitle {
    NSStatusBarButton *button = self.statusItem.button;
    if (button == nil) {
        return;
    }

    button.accessibilityLabel = @"OpenTamer";
    button.accessibilityIdentifier = @"OpenTamerStatusItem";
    BOOL showIcon = [self showMenuBarIcon];
    self.statusItem.length = showIcon ? OpenTamerStatusItemLengthWithIcon : OpenTamerStatusItemLengthTextOnly;
    button.image = showIcon ? [self openTamerStatusIcon] : nil;
    button.imagePosition = NSImageLeft;
    button.alignment = NSTextAlignmentCenter;
    button.target = self;
    button.action = @selector(togglePrimaryPopover:);

    NSString *title = nil;
    if (![self managementEnabled]) {
        title = @"Off";
    } else if ([[self alertLevel] isEqualToString:@"high"]) {
        title = [NSString stringWithFormat:@"! %@", [self formattedPercent:[self totalCPU]]];
    } else {
        title = [self formattedPercent:[self totalCPU]];
    }
    button.attributedTitle = [self statusTitleWithText:title];
    button.toolTip = @"OpenTamer";
}

- (NSMenuItem *)commandItemWithTitle:(NSString *)title command:(NSString *)command {
    BOOL keepMenuOpen = OpenTamerCommandShouldKeepMenuOpen(command);
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
                                                  action:keepMenuOpen ? nil : @selector(sendCommand:)
                                           keyEquivalent:@""];
    item.target = keepMenuOpen ? nil : self;
    item.enabled = YES;
    item.representedObject = command;
    if (keepMenuOpen) {
        OpenTamerCommandMenuItemView *view = [[OpenTamerCommandMenuItemView alloc] initWithTitle:title];
        view.menuItem = item;
        view.target = self;
        view.action = @selector(sendPersistentCommand:);
        item.view = view;
    }
    return item;
}

- (void)sendCommandString:(NSString *)command {
    if (command.length == 0) {
        return;
    }
    opentamer_menu_command(command.UTF8String);
}

- (void)markMenuItem:(NSMenuItem *)selected exclusiveForCommandPrefix:(NSString *)prefix {
    NSMenu *menu = selected.menu;
    if (menu == nil || prefix.length == 0) {
        [self setCommandItem:selected state:NSControlStateValueOn];
        return;
    }
    for (NSMenuItem *item in menu.itemArray) {
        NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
        if ([command hasPrefix:prefix]) {
            [self setCommandItem:item state:NSControlStateValueOff];
        }
    }
    [self setCommandItem:selected state:NSControlStateValueOn];
}

- (void)setPreferenceOffItemChecked:(BOOL)checked forKey:(NSString *)key inMenu:(NSMenu *)menu {
    if (menu == nil || key.length == 0) {
        return;
    }
    NSString *clearCommand = [NSString stringWithFormat:@"pref-clear|%@", key];
    for (NSMenuItem *item in menu.itemArray) {
        NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
        if ([command isEqualToString:clearCommand]) {
            [self setCommandItem:item state:checked ? NSControlStateValueOn : NSControlStateValueOff];
        }
    }
}

- (void)clearPreferencePresetItemsForKey:(NSString *)key inMenu:(NSMenu *)menu {
    if (menu == nil || key.length == 0) {
        return;
    }
    NSArray *prefixes = @[
        [NSString stringWithFormat:@"pref-duration|%@|", key],
        [NSString stringWithFormat:@"pref-float|%@|", key],
        [NSString stringWithFormat:@"pref-string|%@|", key]
    ];
    for (NSMenuItem *item in menu.itemArray) {
        NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
        for (NSString *prefix in prefixes) {
            if ([command hasPrefix:prefix]) {
                [self setCommandItem:item state:NSControlStateValueOff];
                break;
            }
        }
    }
}

- (void)syncCommandItemView:(NSMenuItem *)item {
    if (![item.view isKindOfClass:OpenTamerCommandMenuItemView.class]) {
        return;
    }
    OpenTamerCommandMenuItemView *view = (OpenTamerCommandMenuItemView *)item.view;
    view.title = item.title;
    view.checked = item.state == NSControlStateValueOn;
    view.enabled = item.enabled;
    [view setNeedsDisplay:YES];
}

- (void)setCommandItem:(NSMenuItem *)item state:(NSControlStateValue)state {
    item.state = state;
    [self syncCommandItemView:item];
}

- (void)setCommandItem:(NSMenuItem *)item enabled:(BOOL)enabled {
    item.enabled = enabled;
    [self syncCommandItemView:item];
}

- (void)updateMenuItem:(NSMenuItem *)item afterCommand:(NSString *)command {
    NSArray *parts = [command componentsSeparatedByString:@"|"];
    if (parts.count == 0) {
        return;
    }
    NSString *kind = parts[0];
    if ([kind isEqualToString:@"pref-bool"] && parts.count >= 3) {
        NSString *key = parts[1];
        BOOL value = [parts[2] boolValue];
        [self setCommandItem:item state:value ? NSControlStateValueOn : NSControlStateValueOff];
        item.representedObject = [NSString stringWithFormat:@"pref-bool|%@|%@", key, value ? @"false" : @"true"];
        return;
    }
    if (([kind isEqualToString:@"pref-duration"] ||
         [kind isEqualToString:@"pref-float"] ||
         [kind isEqualToString:@"pref-string"]) && parts.count >= 2) {
        NSString *key = parts[1];
        [self markMenuItem:item exclusiveForCommandPrefix:[NSString stringWithFormat:@"%@|%@|", kind, key]];
        [self setPreferenceOffItemChecked:NO forKey:key inMenu:item.menu];
        return;
    }
    if ([kind isEqualToString:@"pref-clear"] && parts.count >= 2) {
        NSString *key = parts[1];
        [self markMenuItem:item exclusiveForCommandPrefix:[NSString stringWithFormat:@"pref-clear|%@", key]];
        [self clearPreferencePresetItemsForKey:key inMenu:item.menu];
        return;
    }
    if ([kind isEqualToString:@"rule"] && parts.count >= 3) {
        NSString *mode = parts[1];
        NSString *appKey = parts[2];
        if ([mode isEqualToString:@"track-menu-bar"]) {
            [self setCommandItem:item state:NSControlStateValueOn];
            item.representedObject = [NSString stringWithFormat:@"rule|untrack-menu-bar|%@", appKey];
        } else if ([mode isEqualToString:@"untrack-menu-bar"]) {
            [self setCommandItem:item state:NSControlStateValueOff];
            item.representedObject = [NSString stringWithFormat:@"rule|track-menu-bar|%@", appKey];
        } else if ([mode isEqualToString:@"track-managed"]) {
            [self setCommandItem:item state:NSControlStateValueOn];
            item.representedObject = [NSString stringWithFormat:@"rule|untrack-managed|%@", appKey];
        } else if ([mode isEqualToString:@"untrack-managed"]) {
            [self setCommandItem:item state:NSControlStateValueOff];
            item.representedObject = [NSString stringWithFormat:@"rule|track-managed|%@", appKey];
        }
    }
}

- (BOOL)rowCanPause:(NSDictionary *)row {
    id value = row[@"canPause"];
    return value == nil || value == [NSNull null] ? YES : [value boolValue];
}

- (BOOL)rowCanSlow:(NSDictionary *)row {
    id value = row[@"canSlow"];
    return value == nil || value == [NSNull null] ? YES : [value boolValue];
}

- (BOOL)rowIsSystemBucket:(NSDictionary *)row {
    id value = row[@"systemBucket"];
    return value != nil && value != [NSNull null] && [value boolValue];
}

- (NSMenuItem *)cpuLimitItemWithAppKey:(NSString *)appKey enabled:(BOOL)enabled {
    if (!enabled) {
        return [self disabledItemWithTitle:@"Limit CPU..."];
    }
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Limit CPU..."
                                                  action:nil
                                           keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Limit CPU"];
    [submenu addItem:[self cpuLimitPresetItemWithTitle:@"25%" value:@"25" appKey:appKey]];
    [submenu addItem:[self cpuLimitPresetItemWithTitle:@"10%" value:@"10" appKey:appKey]];
    [submenu addItem:[self cpuLimitPresetItemWithTitle:@"5%" value:@"5" appKey:appKey]];
    [submenu addItem:[self cpuLimitPresetItemWithTitle:@"1%" value:@"1" appKey:appKey]];
    [submenu addItem:[self cpuLimitPresetItemWithTitle:@"0.01%" value:@"0.01" appKey:appKey]];
    [submenu addItem:NSMenuItem.separatorItem];

    NSMenuItem *custom = [[NSMenuItem alloc] initWithTitle:@"Custom..."
                                                    action:@selector(promptForCPULimit:)
                                             keyEquivalent:@""];
    custom.target = self;
    custom.representedObject = appKey;
    [submenu addItem:custom];

    item.submenu = submenu;
    return item;
}

- (NSMenuItem *)cpuLimitPresetItemWithTitle:(NSString *)title value:(NSString *)value appKey:(NSString *)appKey {
    return [self commandItemWithTitle:title command:[NSString stringWithFormat:@"rule|limit|%@|%@", value, appKey]];
}

- (NSMenuItem *)priorityMenuItemWithAppKey:(NSString *)appKey enabled:(BOOL)enabled {
    if (!enabled) {
        return [self disabledItemWithTitle:@"Lower Priority..."];
    }
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Lower Priority..."
                                                  action:nil
                                           keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Lower Priority"];
    [submenu addItem:[self commandItemWithTitle:@"In Background" command:[NSString stringWithFormat:@"rule|priority-background|%@", appKey]]];
    [submenu addItem:[self commandItemWithTitle:@"Always" command:[NSString stringWithFormat:@"rule|priority-always|%@", appKey]]];
    item.submenu = submenu;
    return item;
}

- (NSMenuItem *)trackingMenuItemForRow:(NSDictionary *)row {
    NSString *appKey = [self appKeyFromRow:row];
    if (appKey.length == 0 || [self rowIsSystemBucket:row]) {
        return [self disabledItemWithTitle:@"Track in..."];
    }

    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Track in..."
                                                  action:nil
                                           keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Track in"];

    BOOL trackedInMenuBar = [row[@"trackedInMenuBar"] boolValue];
    BOOL trackedInManagedApps = [row[@"trackedInManagedApps"] boolValue];

    NSString *menuBarCommand = trackedInMenuBar
        ? [NSString stringWithFormat:@"rule|untrack-menu-bar|%@", appKey]
        : [NSString stringWithFormat:@"rule|track-menu-bar|%@", appKey];
    NSMenuItem *menuBar = [self commandItemWithTitle:@"The Menu Bar" command:menuBarCommand];
    [self setCommandItem:menuBar state:trackedInMenuBar ? NSControlStateValueOn : NSControlStateValueOff];
    [submenu addItem:menuBar];

    NSString *managedCommand = trackedInManagedApps
        ? [NSString stringWithFormat:@"rule|untrack-managed|%@", appKey]
        : [NSString stringWithFormat:@"rule|track-managed|%@", appKey];
    NSMenuItem *managed = [self commandItemWithTitle:@"Tracked Processes" command:managedCommand];
    [self setCommandItem:managed state:trackedInManagedApps ? NSControlStateValueOn : NSControlStateValueOff];
    [submenu addItem:managed];

    item.submenu = submenu;
    return item;
}

- (NSMenu *)trackedStatusMenuForRow:(NSDictionary *)row {
    NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
    NSString *status = [self stringFromValue:row[@"status"] fallback:@""];
    NSString *blocker = [self stringFromValue:row[@"blockerReason"] fallback:@""];
    NSString *appKey = [self appKeyFromRow:row];

    NSMenu *menu = [[OpenTamerPersistentMenu alloc] initWithTitle:name];
    [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"%@  %@", name, [self cpuTitleForRow:row]]]];
    [self addCPUSummaryForRow:row toMenu:menu];
    [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Status: %@", status.length == 0 ? @"unknown" : status]]];
    if (blocker.length > 0) {
        [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Note: %@", blocker]]];
    }
    [menu addItem:NSMenuItem.separatorItem];
    [menu addItem:[self commandItemWithTitle:@"Untrack" command:[NSString stringWithFormat:@"rule|untrack-menu-bar|%@", appKey]]];
    [menu addItem:NSMenuItem.separatorItem];
    [menu addItem:[self trackingMenuItemForRow:row]];
    [menu addItem:[self priorityMenuItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    [menu addItem:[self cpuLimitItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    NSMenuItem *pause = [self commandItemWithTitle:@"Pause in Background" command:[NSString stringWithFormat:@"rule|pause|%@", appKey]];
    [self setCommandItem:pause enabled:[self rowCanPause:row]];
    [menu addItem:pause];
    return menu;
}

- (void)updateTrackedStatusItems {
    NSMutableSet *seen = [NSMutableSet set];
    for (id rowObject in [self menuBarApps]) {
        if (![rowObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        NSDictionary *row = rowObject;
        NSString *appKey = [self appKeyFromRow:row];
        if (appKey.length == 0) {
            continue;
        }
        [seen addObject:appKey];

        NSStatusItem *item = self.trackedStatusItems[appKey];
        if (item == nil) {
            item = [NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength];
            self.trackedStatusItems[appKey] = item;
        }

        NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
        double displayCPU = [self displayCPUPercentForRow:row];
        item.button.title = [NSString stringWithFormat:@"%@ %@", [self shortStatusName:name], [self formattedStatusPercent:displayCPU]];
        item.button.toolTip = [NSString stringWithFormat:@"%@ %@ %@",
                               name,
                               [[self cpuDisplayLabel] lowercaseString],
                               [self formattedStatusPercent:displayCPU]];
        item.menu = [self trackedStatusMenuForRow:row];
    }

    for (NSString *appKey in [self.trackedStatusItems.allKeys copy]) {
        if ([seen containsObject:appKey]) {
            continue;
        }
        NSStatusItem *item = self.trackedStatusItems[appKey];
        if (item != nil) {
            [NSStatusBar.systemStatusBar removeStatusItem:item];
        }
        [self.trackedStatusItems removeObjectForKey:appKey];
    }
}

- (NSMenuItem *)disabledItemWithTitle:(NSString *)title {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    item.enabled = NO;
    return item;
}

- (NSMenu *)processMenuForRow:(NSDictionary *)row {
    NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
    NSString *status = [self stringFromValue:row[@"status"] fallback:@""];
    NSString *blocker = [self stringFromValue:row[@"blockerReason"] fallback:@""];

    NSString *appKey = [self appKeyFromRow:row];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:name];
    [self addCPUSummaryForRow:row toMenu:submenu];
    if ([self rowIsSystemBucket:row]) {
        if (blocker.length > 0) {
            [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Note: %@", blocker]]];
        }
        return submenu;
    }
    [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Status: %@", status.length == 0 ? @"unknown" : status]]];
    if (blocker.length > 0) {
        [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Note: %@", blocker]]];
    }
    [submenu addItem:NSMenuItem.separatorItem];
    [submenu addItem:[self trackingMenuItemForRow:row]];
    [submenu addItem:[self priorityMenuItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    [submenu addItem:[self cpuLimitItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    NSMenuItem *pause = [self commandItemWithTitle:@"Pause in Background" command:[NSString stringWithFormat:@"rule|pause|%@", appKey]];
    [self setCommandItem:pause enabled:[self rowCanPause:row]];
    [submenu addItem:pause];
    return submenu;
}

- (NSMenuItem *)processItemForRow:(NSDictionary *)row {
    NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
    int pid = [row[@"pid"] intValue];
    NSString *title = pid > 0
        ? [NSString stringWithFormat:@"%@  %@  PID %d", name, [self cpuTitleForRow:row], pid]
        : [NSString stringWithFormat:@"%@  %@", name, [self cpuTitleForRow:row]];
    if ([self rowIsSystemBucket:row]) {
        return [self disabledItemWithTitle:title];
    }
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [self processMenuForRow:row];
    item.submenu = submenu;
    return item;
}

- (void)addSummaryToMenu:(NSMenu *)menu {
    NSString *summary = [NSString stringWithFormat:@"System CPU: %@",
                         [self formattedPercent:[self totalCPU]]];
    [menu addItem:[self disabledItemWithTitle:summary]];

    NSString *status = [self statusMessage];
    if (status.length > 0) {
        [menu addItem:[self disabledItemWithTitle:status]];
    }
}

- (void)addTopProcessesToMenu:(NSMenu *)menu {
    [menu addItem:[self disabledItemWithTitle:[self topProcessesHeaderTitle]]];

    NSArray *rows = [self topProcesses];
    if (rows.count == 0) {
        [menu addItem:[self disabledItemWithTitle:@"No process samples yet"]];
        return;
    }

    for (id rowObject in rows) {
        if (![rowObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        [menu addItem:[self processItemForRow:rowObject]];
    }
}

- (void)addTrackedAppsToMenu:(NSMenu *)menu {
    [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Tracked Processes (current %@)", [self topProcessesCPULabel]]]];

    NSArray *rows = [self trackedApps];
    if (rows.count == 0) {
        [menu addItem:[self disabledItemWithTitle:@"No tracked processes configured"]];
        return;
    }

    for (id rowObject in rows) {
        if (![rowObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        [menu addItem:[self processItemForRow:rowObject]];
    }
}

- (NSMenu *)allProcessesMenu {
    NSArray *rows = [self allProcesses];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"All Processes"];

    if (rows.count == 0) {
        [submenu addItem:[self disabledItemWithTitle:@"No processes observed yet"]];
    } else {
        for (id rowObject in rows) {
            if (![rowObject isKindOfClass:NSDictionary.class]) {
                continue;
            }
            [submenu addItem:[self processItemForRow:rowObject]];
        }
    }

    return submenu;
}

- (void)addAllProcessesToMenu:(NSMenu *)menu {
    NSArray *rows = [self allProcesses];
    NSString *title = [NSString stringWithFormat:@"All Processes (%lu, current %@)", (unsigned long)rows.count, [self topProcessesCPULabel]];
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [self allProcessesMenu];
    item.submenu = submenu;
    [menu addItem:item];
}

- (NSMenu *)managedMenuForRow:(NSDictionary *)row {
    NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
    NSString *mode = [self stringFromValue:row[@"ruleLabel"] fallback:[self stringFromValue:row[@"ruleMode"] fallback:@"observe_only"]];
    NSString *status = [self stringFromValue:row[@"status"] fallback:@"unknown"];
    NSString *blocker = [self stringFromValue:row[@"blockerReason"] fallback:@""];
    NSString *appKey = [self appKeyFromRow:row];

    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:name];
    [self addCPUSummaryForRow:row toMenu:submenu];
    [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Status: %@", status]]];
    [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Rule: %@", mode]]];
    if (blocker.length > 0) {
        [submenu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Note: %@", blocker]]];
    }
    [submenu addItem:NSMenuItem.separatorItem];
    [submenu addItem:[self trackingMenuItemForRow:row]];
    [submenu addItem:[self priorityMenuItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    [submenu addItem:[self cpuLimitItemWithAppKey:appKey enabled:[self rowCanSlow:row]]];
    NSMenuItem *pause = [self commandItemWithTitle:@"Pause in Background" command:[NSString stringWithFormat:@"rule|pause|%@", appKey]];
    [self setCommandItem:pause enabled:[self rowCanPause:row]];
    [submenu addItem:pause];
    [submenu addItem:NSMenuItem.separatorItem];
    [submenu addItem:[self commandItemWithTitle:@"Remove Rule" command:[NSString stringWithFormat:@"disable-rule|%@", appKey]]];
    return submenu;
}

- (void)addManagedAppsToMenu:(NSMenu *)menu {
    [menu addItem:[self disabledItemWithTitle:[NSString stringWithFormat:@"Managed Apps (current %@)", [self topProcessesCPULabel]]]];

    NSArray *rows = [self managedApps];
    if (rows.count == 0) {
        [menu addItem:[self disabledItemWithTitle:@"No app rules configured"]];
        return;
    }

    for (id rowObject in rows) {
        if (![rowObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        NSDictionary *row = rowObject;
        NSString *name = [self stringFromValue:row[@"name"] fallback:@"Unknown"];
        NSString *mode = [self stringFromValue:row[@"ruleLabel"] fallback:[self stringFromValue:row[@"ruleMode"] fallback:@"observe_only"]];

        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:[NSString stringWithFormat:@"%@  %@  %@", name, [self cpuTitleForRow:row], mode]
                                                      action:nil
                                               keyEquivalent:@""];
        item.submenu = [self managedMenuForRow:row];
        [menu addItem:item];
    }
}

- (NSMenuItem *)boolPreferenceItemWithTitle:(NSString *)title key:(NSString *)key fallback:(BOOL)fallback {
    BOOL current = [self boolPreference:key fallback:fallback];
    NSString *next = current ? @"false" : @"true";
    NSMenuItem *item = [self commandItemWithTitle:title command:[NSString stringWithFormat:@"pref-bool|%@|%@", key, next]];
    [self setCommandItem:item state:current ? NSControlStateValueOn : NSControlStateValueOff];
    return item;
}

- (NSMenuItem *)launchAtLoginItem {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Launch at Login"
                                                  action:@selector(toggleLaunchAtLogin:)
                                           keyEquivalent:@""];
    item.target = self;
    item.enabled = YES;

    if (@available(macOS 13.0, *)) {
        SMAppServiceStatus status = SMAppService.mainAppService.status;
        if (status == SMAppServiceStatusEnabled) {
            item.state = NSControlStateValueOn;
        } else if (status == SMAppServiceStatusRequiresApproval) {
            item.state = NSControlStateValueMixed;
            item.toolTip = @"Approve OpenTamer in System Settings to launch it at login.";
        } else {
            item.state = NSControlStateValueOff;
        }
    } else {
        item.enabled = NO;
    }
    return item;
}

- (long long)nanosecondsForSeconds:(double)seconds {
    return (long long)llround(seconds * 1000000000.0);
}

- (NSMenuItem *)durationPreferencePresetItemWithTitle:(NSString *)title
                                                 key:(NSString *)key
                                             seconds:(double)seconds
                                            fallback:(double)fallback {
    long long nanoseconds = [self nanosecondsForSeconds:seconds];
    NSMenuItem *item = [self commandItemWithTitle:title command:[NSString stringWithFormat:@"pref-duration|%@|%lld", key, nanoseconds]];
    double current = [self durationPreferenceSeconds:key fallback:fallback];
    double tolerance = seconds < 1 ? 0.001 : 0.5;
    [self setCommandItem:item state:fabs(current - seconds) <= tolerance ? NSControlStateValueOn : NSControlStateValueOff];
    return item;
}

- (void)addDurationPreferenceWithTitle:(NSString *)title
                                   key:(NSString *)key
                              fallback:(double)fallback
                                labels:(NSArray *)labels
                               seconds:(NSArray *)seconds
                                 toMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:title];
    NSUInteger count = MIN(labels.count, seconds.count);
    for (NSUInteger index = 0; index < count; index++) {
        NSString *label = [labels[index] isKindOfClass:NSString.class] ? labels[index] : @"";
        NSNumber *value = [seconds[index] respondsToSelector:@selector(doubleValue)] ? seconds[index] : @(fallback);
        if (label.length == 0) {
            continue;
        }
        [submenu addItem:[self durationPreferencePresetItemWithTitle:label key:key seconds:value.doubleValue fallback:fallback]];
    }
    item.submenu = submenu;
    [menu addItem:item];
}

- (NSMenuItem *)floatPreferencePresetItemWithTitle:(NSString *)title
                                               key:(NSString *)key
                                             value:(double)value
                                          fallback:(double)fallback {
    NSMenuItem *item = [self commandItemWithTitle:title command:[NSString stringWithFormat:@"pref-float|%@|%.6g", key, value]];
    double current = [self floatPreference:key fallback:fallback];
    [self setCommandItem:item state:fabs(current - value) <= 0.001 ? NSControlStateValueOn : NSControlStateValueOff];
    return item;
}

- (void)addFloatPreferenceWithTitle:(NSString *)title
                                key:(NSString *)key
                           fallback:(double)fallback
                             labels:(NSArray *)labels
                             values:(NSArray *)values
                               toMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:title];
    NSUInteger count = MIN(labels.count, values.count);
    for (NSUInteger index = 0; index < count; index++) {
        NSString *label = [labels[index] isKindOfClass:NSString.class] ? labels[index] : @"";
        NSNumber *value = [values[index] respondsToSelector:@selector(doubleValue)] ? values[index] : @(fallback);
        if (label.length == 0) {
            continue;
        }
        [submenu addItem:[self floatPreferencePresetItemWithTitle:label key:key value:value.doubleValue fallback:fallback]];
    }
    item.submenu = submenu;
    [menu addItem:item];
}

- (void)addHighCPUThresholdPreferenceToMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Threshold" action:nil keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Threshold"];
    NSArray *labels = @[@"50%", @"75%", @"90%", @"100%"];
    NSArray *values = @[@50, @75, @90, @100];
    for (NSUInteger index = 0; index < labels.count; index++) {
        NSString *label = [labels[index] isKindOfClass:NSString.class] ? labels[index] : @"";
        NSNumber *value = [values[index] respondsToSelector:@selector(doubleValue)] ? values[index] : @75;
        if (label.length == 0) {
            continue;
        }
        [submenu addItem:[self floatPreferencePresetItemWithTitle:label key:@"highCPUThreshold" value:value.doubleValue fallback:75]];
    }
    [submenu addItem:NSMenuItem.separatorItem];

    NSMenuItem *custom = [[NSMenuItem alloc] initWithTitle:@"Custom..."
                                                    action:@selector(promptForHighCPUThreshold:)
                                             keyEquivalent:@""];
    custom.target = self;
    [submenu addItem:custom];

    item.submenu = submenu;
    [menu addItem:item];
}

- (NSMenuItem *)stringPreferencePresetItemWithTitle:(NSString *)title
                                                key:(NSString *)key
                                              value:(NSString *)value
                                           fallback:(NSString *)fallback {
    NSMenuItem *item = [self commandItemWithTitle:title command:[NSString stringWithFormat:@"pref-string|%@|%@", key, value]];
    NSString *current = [self stringPreference:key fallback:fallback];
    [self setCommandItem:item state:[current isEqualToString:value] ? NSControlStateValueOn : NSControlStateValueOff];
    return item;
}

- (void)addStringPreferenceWithTitle:(NSString *)title
                                  key:(NSString *)key
                             fallback:(NSString *)fallback
                               labels:(NSArray *)labels
                               values:(NSArray *)values
                               toMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:title];
    NSUInteger count = MIN(labels.count, values.count);
    for (NSUInteger index = 0; index < count; index++) {
        NSString *label = [labels[index] isKindOfClass:NSString.class] ? labels[index] : @"";
        NSString *value = [values[index] isKindOfClass:NSString.class] ? values[index] : fallback;
        if (label.length == 0 || value.length == 0) {
            continue;
        }
        [submenu addItem:[self stringPreferencePresetItemWithTitle:label key:key value:value fallback:fallback]];
    }
    item.submenu = submenu;
    [menu addItem:item];
}

- (void)addNullableFloatPreferenceWithTitle:(NSString *)title
                                        key:(NSString *)key
                                     labels:(NSArray *)labels
                                     values:(NSArray *)values
                                     toMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:title];

    NSMenuItem *off = [self commandItemWithTitle:@"Off" command:[NSString stringWithFormat:@"pref-clear|%@", key]];
    [self setCommandItem:off state:[self hasPreference:key] ? NSControlStateValueOff : NSControlStateValueOn];
    [submenu addItem:off];
    [submenu addItem:NSMenuItem.separatorItem];

    NSUInteger count = MIN(labels.count, values.count);
    for (NSUInteger index = 0; index < count; index++) {
        NSString *label = [labels[index] isKindOfClass:NSString.class] ? labels[index] : @"";
        NSNumber *value = [values[index] respondsToSelector:@selector(doubleValue)] ? values[index] : @0;
        if (label.length == 0) {
            continue;
        }
        NSMenuItem *preset = [self floatPreferencePresetItemWithTitle:label key:key value:value.doubleValue fallback:NAN];
        if (![self hasPreference:key]) {
            [self setCommandItem:preset state:NSControlStateValueOff];
        }
        [submenu addItem:preset];
    }

    item.submenu = submenu;
    [menu addItem:item];
}

- (NSMenu *)preferencesMenu {
    NSMenu *submenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Preferences"];

    NSMenuItem *general = [[NSMenuItem alloc] initWithTitle:@"General" action:nil keyEquivalent:@""];
    NSMenu *generalMenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"General"];
    [generalMenu addItem:[self launchAtLoginItem]];
    [generalMenu addItem:[self boolPreferenceItemWithTitle:@"Show Menu Icon" key:@"showMenuBarIcon" fallback:[self showMenuBarIcon]]];
    [generalMenu addItem:[self boolPreferenceItemWithTitle:@"Aggregate By Name" key:@"aggregateByName" fallback:YES]];
    [self addStringPreferenceWithTitle:@"CPU Display"
                                    key:@"cpuDisplayMode"
                               fallback:@"per_core_process"
                                 labels:@[@"Per-Core Process CPU", @"System Normalized CPU"]
                                 values:@[@"per_core_process", @"system_normalized"]
                                 toMenu:generalMenu];
    [self addDurationPreferenceWithTitle:@"Wake Grace"
                                     key:@"wakeGrace"
                                fallback:30
                                  labels:@[@"Off", @"10 seconds", @"30 seconds", @"1 minute", @"5 minutes"]
                                 seconds:@[@0, @10, @30, @60, @300]
                                  toMenu:generalMenu];
    general.submenu = generalMenu;
    [submenu addItem:general];

    NSMenuItem *stats = [[NSMenuItem alloc] initWithTitle:@"Stats & Graph" action:nil keyEquivalent:@""];
    NSMenu *statsMenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Stats & Graph"];
    [self addDurationPreferenceWithTitle:@"Stats Interval"
                                     key:@"statsInterval"
                                fallback:3
                                  labels:@[@"3 seconds", @"5 seconds", @"10 seconds", @"30 seconds", @"1 minute"]
                                 seconds:@[@3, @5, @10, @30, @60]
                                  toMenu:statsMenu];
    [self addDurationPreferenceWithTitle:@"Averaging Window"
                                     key:@"averagingWindow"
                                fallback:30
                                  labels:@[@"10 seconds", @"30 seconds", @"1 minute", @"2 minutes", @"5 minutes"]
                                 seconds:@[@10, @30, @60, @120, @300]
                                  toMenu:statsMenu];
    [self addDurationPreferenceWithTitle:@"CPU Graph Window"
                                     key:@"cpuGraphWindow"
                                fallback:300
                                  labels:@[@"1 minute", @"5 minutes", @"10 minutes", @"30 minutes"]
                                 seconds:@[@60, @300, @600, @1800]
                                  toMenu:statsMenu];
    [self addStringPreferenceWithTitle:@"Top Processes Sort"
                                    key:@"topProcessesSort"
                               fallback:@"current"
                                 labels:@[@"Current CPU", @"60s Average"]
                                 values:@[@"current", @"average"]
                                 toMenu:statsMenu];
    stats.submenu = statsMenu;
    [submenu addItem:stats];

    NSMenuItem *alerts = [[NSMenuItem alloc] initWithTitle:@"High CPU Alerts" action:nil keyEquivalent:@""];
    NSMenu *alertsMenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"High CPU Alerts"];
    [alertsMenu addItem:[self boolPreferenceItemWithTitle:@"Detection Enabled" key:@"highCPUDetectionEnabled" fallback:YES]];
    [self addHighCPUThresholdPreferenceToMenu:alertsMenu];
    [self addDurationPreferenceWithTitle:@"Alert After"
                                     key:@"highCPUDuration"
                                fallback:30
                                  labels:@[@"10 seconds", @"30 seconds", @"1 minute", @"5 minutes"]
                                 seconds:@[@10, @30, @60, @300]
                                  toMenu:alertsMenu];
    [self addDurationPreferenceWithTitle:@"Cooldown"
                                     key:@"highCPUCooldown"
                                fallback:600
                                  labels:@[@"1 minute", @"5 minutes", @"10 minutes", @"30 minutes", @"1 hour"]
                                 seconds:@[@60, @300, @600, @1800, @3600]
                                  toMenu:alertsMenu];
    alerts.submenu = alertsMenu;
    [submenu addItem:alerts];

    NSMenuItem *policy = [[NSMenuItem alloc] initWithTitle:@"Policy Guards" action:nil keyEquivalent:@""];
    NSMenu *policyMenu = [[OpenTamerPersistentMenu alloc] initWithTitle:@"Policy Guards"];
    [self addNullableFloatPreferenceWithTitle:@"Disable On AC Battery Above"
                                          key:@"disableWhenACBatteryAbove"
                                       labels:@[@"50%", @"75%", @"90%", @"100%"]
                                       values:@[@50, @75, @90, @100]
                                       toMenu:policyMenu];
    [self addDurationPreferenceWithTitle:@"Disable When User Idle Longer Than"
                                     key:@"disableWhenUserIdleLongerThan"
                                fallback:0
                                  labels:@[@"Off", @"5 minutes", @"15 minutes", @"30 minutes", @"1 hour"]
                                 seconds:@[@0, @300, @900, @1800, @3600]
                                  toMenu:policyMenu];
    policy.submenu = policyMenu;
    [submenu addItem:policy];

    [submenu addItem:[self commandItemWithTitle:@"Reset Defaults" command:@"reset-defaults"]];

    [submenu addItem:NSMenuItem.separatorItem];
    NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"Quit OpenTamer"
                                                  action:@selector(quit:)
                                           keyEquivalent:@"q"];
    quit.target = self;
    [submenu addItem:quit];
    return submenu;
}

- (NSMenuItem *)graphWindowPresetItemWithTitle:(NSString *)title duration:(double)seconds {
    long long nanoseconds = (long long)(seconds * 1000000000.0);
    NSMenuItem *item = [self commandItemWithTitle:title command:[NSString stringWithFormat:@"graph-window|%lld", nanoseconds]];
    NSDictionary *graph = [self cpuGraph];
    double current = OpenTamerDoubleFromRow(graph, @"windowEndUnix", 0) - OpenTamerDoubleFromRow(graph, @"windowStartUnix", 0);
    [self setCommandItem:item state:fabs(current - seconds) < 0.5 ? NSControlStateValueOn : NSControlStateValueOff];
    return item;
}

- (void)addPreferencesToMenu:(NSMenu *)menu {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"Preferences" action:nil keyEquivalent:@""];
    NSMenu *submenu = [self preferencesMenu];
    item.submenu = submenu;
    [menu addItem:item];
}

- (NSTextField *)labelWithFrame:(NSRect)frame
                          title:(NSString *)title
                           font:(NSFont *)font
                          color:(NSColor *)color
                      alignment:(NSTextAlignment)alignment {
    NSTextField *label = [[NSTextField alloc] initWithFrame:frame];
    label.stringValue = title ?: @"";
    label.font = font;
    label.textColor = color;
    label.alignment = alignment;
    label.bezeled = NO;
    label.drawsBackground = NO;
    label.editable = NO;
    label.selectable = NO;
    label.lineBreakMode = NSLineBreakByTruncatingTail;
    return label;
}

- (NSDictionary *)graphColorIndexesByAppKey {
    NSArray *lines = [self cpuGraphLines];
    NSMutableDictionary *indexes = [NSMutableDictionary dictionaryWithCapacity:lines.count];
    NSUInteger lineIndex = 0;
    for (id lineObject in lines) {
        if (![lineObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        NSDictionary *line = lineObject;
        NSString *appKey = OpenTamerStringFromValue(line[@"appKey"], @"");
        if (appKey.length > 0 && indexes[appKey] == nil) {
            indexes[appKey] = @(lineIndex);
        }
        lineIndex++;
    }
    return indexes;
}

- (void)addSectionHeaderToView:(NSView *)view
                             y:(CGFloat *)y
                         width:(CGFloat)width
                       padding:(CGFloat)padding
                         title:(NSString *)title
                    showColumns:(BOOL)showColumns {
    NSTextField *titleLabel = [self labelWithFrame:NSMakeRect(padding, *y, width - padding * 2, 18)
                                             title:title
                                              font:[NSFont systemFontOfSize:11 weight:NSFontWeightSemibold]
                                             color:[NSColor secondaryLabelColor]
                                         alignment:NSTextAlignmentLeft];
    [view addSubview:titleLabel];
    if (showColumns) {
        BOOL topProcessesHeader = [title isEqualToString:@"Top Processes"];
        BOOL sortAverage = [[self topProcessesSortMode] isEqualToString:@"average"];
        NSFont *cpuFont = topProcessesHeader && !sortAverage
            ? [NSFont systemFontOfSize:10 weight:NSFontWeightSemibold]
            : [NSFont systemFontOfSize:10 weight:NSFontWeightMedium];
        NSFont *avgFont = topProcessesHeader && sortAverage
            ? [NSFont systemFontOfSize:10 weight:NSFontWeightSemibold]
            : [NSFont systemFontOfSize:10 weight:NSFontWeightMedium];
        NSColor *cpuColor = topProcessesHeader && !sortAverage ? [NSColor secondaryLabelColor] : [NSColor tertiaryLabelColor];
        NSColor *avgColor = topProcessesHeader && sortAverage ? [NSColor secondaryLabelColor] : [NSColor tertiaryLabelColor];
        NSRect cpuFrame = NSMakeRect(width - padding - 116, *y, 52, 18);
        NSRect avgFrame = NSMakeRect(width - padding - 58, *y, 52, 18);
        NSTextField *cpu = [self labelWithFrame:cpuFrame
                                          title:@"CPU"
                                           font:cpuFont
                                          color:cpuColor
                                      alignment:NSTextAlignmentRight];
        NSTextField *avg = [self labelWithFrame:avgFrame
                                          title:@"Avg"
                                           font:avgFont
                                          color:avgColor
                                      alignment:NSTextAlignmentRight];
        [view addSubview:cpu];
        [view addSubview:avg];
        if (topProcessesHeader && [view isKindOfClass:OpenTamerPanelView.class]) {
            OpenTamerPanelView *panel = (OpenTamerPanelView *)view;
            [panel addActionNamed:@"sort-current" frame:cpuFrame row:nil kind:nil title:nil];
            [panel addActionNamed:@"sort-average" frame:avgFrame row:nil kind:nil title:nil];
        }
    }
    *y += 20;
}

- (void)addProcessRows:(NSArray *)rows
                toView:(OpenTamerPanelView *)view
                     y:(CGFloat *)y
                 width:(CGFloat)width
               padding:(CGFloat)padding
              rowCount:(NSUInteger)rowCount
                  kind:(NSString *)kind
 graphColorIndexesByAppKey:(NSDictionary *)graphColorIndexesByAppKey
             emptyText:(NSString *)emptyText {
    NSUInteger added = 0;
    for (id rowObject in rows) {
        if (added >= rowCount) {
            break;
        }
        if (![rowObject isKindOfClass:NSDictionary.class]) {
            continue;
        }
        NSRect rowFrame = NSMakeRect(padding, *y, width - padding * 2, 24);
        NSDictionary *row = rowObject;
        BOOL systemBucket = [self rowIsSystemBucket:row];
        NSString *actionName = systemBucket ? @"process-static" : ([kind isEqualToString:@"managed"] ? @"managed" : @"process");
        [view addActionNamed:actionName frame:rowFrame row:rowObject kind:kind title:nil];
        NSMutableDictionary *rowAction = view.actions.lastObject;
        rowAction[@"cpuDisplayMode"] = [self cpuDisplayMode];
        if (graphColorIndexesByAppKey.count > 0) {
            NSString *appKey = OpenTamerStringFromValue(row[@"appKey"], @"");
            NSNumber *graphColorIndex = graphColorIndexesByAppKey[appKey];
            if (graphColorIndex != nil) {
                rowAction[@"graphColorIndex"] = graphColorIndex;
                rowAction[@"graphLineHidden"] = @([self isGraphLineHiddenForAppKey:appKey]);

                NSRect dotFrame = NSMakeRect(NSMinX(rowFrame) + 4, NSMinY(rowFrame) + 2, 24, 20);
                [view addActionNamed:@"graph-toggle" frame:dotFrame row:nil kind:nil title:nil];
                NSMutableDictionary *dotAction = view.actions.lastObject;
                dotAction[@"appKey"] = appKey;
                dotAction[@"graphColorIndex"] = graphColorIndex;
            }
        }
        *y += 24;
        added++;
    }

    if (added == 0) {
        NSTextField *empty = [self labelWithFrame:NSMakeRect(padding + 8, *y + 2, width - padding * 2 - 16, 18)
                                            title:emptyText
                                             font:[NSFont systemFontOfSize:11]
                                            color:[NSColor tertiaryLabelColor]
                                        alignment:NSTextAlignmentLeft];
        [view addSubview:empty];
        *y += 24;
    }
}

- (OpenTamerPanelView *)primaryPanelView {
    CGFloat width = 370;
    CGFloat padding = 14;
    NSUInteger topCount = MIN((NSUInteger)10, [self topProcesses].count);
    NSUInteger trackedCount = MIN((NSUInteger)2, [self trackedApps].count);
    NSUInteger managedCount = MIN((NSUInteger)2, [self managedApps].count);
    CGFloat height = 12 + 30 + 8 + 118 + 20 + MAX((NSUInteger)1, topCount) * 24 + 10 + 28 + 20 + MAX((NSUInteger)1, trackedCount) * 24 + 8 + 20 + MAX((NSUInteger)1, managedCount) * 24 + 30;

    OpenTamerPanelView *view = [[OpenTamerPanelView alloc] initWithFrame:NSMakeRect(0, 0, width, height)];
    view.controller = (id<OpenTamerPanelActionHandling>)self;
    CGFloat y = 12;
    NSDictionary *graphColorIndexesByAppKey = [self graphColorIndexesByAppKey];

    NSRect toggleFrame = NSMakeRect(padding, y + 3, 46, 24);
    [view addActionNamed:@"toggle" frame:toggleFrame row:nil kind:nil title:nil];
    NSMutableDictionary *toggleAction = view.actions.lastObject;
    toggleAction[@"toggleOn"] = @([self managementEnabled]);

    NSTextField *title = [self labelWithFrame:NSMakeRect(padding + 58, y + 4, 142, 22)
                                        title:@"OpenTamer"
                                         font:[NSFont systemFontOfSize:17 weight:NSFontWeightSemibold]
                                        color:[NSColor labelColor]
                                    alignment:NSTextAlignmentLeft];
    [view addSubview:title];

    NSRect refreshFrame = NSMakeRect(width - padding - 104, y + 2, 68, 26);
    [view addActionNamed:@"refresh" frame:refreshFrame row:nil kind:nil title:@"Refresh"];

    NSRect prefsFrame = NSMakeRect(width - padding - 30, y + 2, 30, 26);
    [view addActionNamed:@"preferences" frame:prefsFrame row:nil kind:nil title:@"..."];

    y += 38;

    NSString *summary = [NSString stringWithFormat:@"System CPU %@   %@",
                         [self formattedStatusPercent:[self totalCPU]],
                         [self statusMessage]];
    NSTextField *summaryLabel = [self labelWithFrame:NSMakeRect(padding, y, width - padding * 2, 16)
                                               title:summary
                                                font:[NSFont systemFontOfSize:11]
                                               color:[NSColor secondaryLabelColor]
                                           alignment:NSTextAlignmentLeft];
    [view addSubview:summaryLabel];
    y += 18;

    OpenTamerCPUGraphView *graph = [[OpenTamerCPUGraphView alloc] initWithFrame:NSMakeRect(padding, y, width - padding * 2, 118)];
    graph.lines = [self cpuGraphLines];
    graph.hiddenAppKeys = [self.hiddenGraphAppKeys copy];
    graph.currentCPU = [self cpuGraphCurrentCPU];
    NSDictionary *graphState = [self cpuGraph];
    graph.windowStartUnix = OpenTamerDoubleFromRow(graphState, @"windowStartUnix", 0);
    graph.windowEndUnix = OpenTamerDoubleFromRow(graphState, @"windowEndUnix", 0);
    [view addSubview:graph];
    y += 122;

    [self addSectionHeaderToView:view y:&y width:width padding:padding title:@"Top Processes" showColumns:YES];
    [self addProcessRows:[self topProcesses] toView:view y:&y width:width padding:padding rowCount:10 kind:@"process" graphColorIndexesByAppKey:graphColorIndexesByAppKey emptyText:@"No process samples yet"];

    y += 8;
    NSRect allFrame = NSMakeRect(padding, y, width - padding * 2, 26);
    [view addActionNamed:@"all-processes"
                   frame:allFrame
                     row:nil
                    kind:nil
                   title:[NSString stringWithFormat:@"All Processes (%lu) >", (unsigned long)[self allProcesses].count]];
    y += 34;

    [self addSectionHeaderToView:view y:&y width:width padding:padding title:@"Tracked Processes" showColumns:YES];
    [self addProcessRows:[self trackedApps] toView:view y:&y width:width padding:padding rowCount:2 kind:@"process" graphColorIndexesByAppKey:nil emptyText:@"No tracked processes"];

    y += 8;
    [self addSectionHeaderToView:view y:&y width:width padding:padding title:@"Managed Processes" showColumns:NO];
    [self addProcessRows:[self managedApps] toView:view y:&y width:width padding:padding rowCount:2 kind:@"managed" graphColorIndexesByAppKey:nil emptyText:@"No managed processes"];

    [view installActionButtonsWithTarget:self action:@selector(performPanelButtonAction:)];

    return view;
}

- (NSMenuItem *)primaryPanelMenuItem {
    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:@"" action:nil keyEquivalent:@""];
    item.enabled = YES;
    item.view = [self primaryPanelView];
    return item;
}

- (void)rebuildMenu {
    self.needsMenuRebuild = NO;
    if (self.statusItem != nil) {
        self.statusItem.menu = nil;
    }
}

- (void)togglePrimaryPopover:(id)sender {
    if (self.primaryPopover.isShown) {
        [self.primaryPopover performClose:sender];
        return;
    }
    [self showPrimaryPopover];
}

- (void)showPrimaryPopover {
    NSStatusBarButton *button = self.statusItem.button;
    if (button == nil) {
        return;
    }

    NSPopover *popover = [[NSPopover alloc] init];
    popover.behavior = NSPopoverBehaviorTransient;
    popover.animates = NO;
    popover.delegate = self;
    popover.appearance = [NSAppearance appearanceNamed:NSAppearanceNameAqua];

    NSViewController *viewController = [[NSViewController alloc] init];
    viewController.view = [self primaryPanelView];
    popover.contentViewController = viewController;

    self.primaryPopover = popover;
    self.menuVisible = YES;
    self.needsMenuRebuild = NO;
    [popover showRelativeToRect:button.bounds ofView:button preferredEdge:NSMinYEdge];
    if (viewController.view.window != nil) {
        [viewController.view.window makeFirstResponder:viewController.view];
    }
}

- (void)refreshPrimaryPopoverContent {
    if (!self.primaryPopover.isShown) {
        return;
    }
    NSViewController *viewController = self.primaryPopover.contentViewController;
    if (viewController == nil) {
        viewController = [[NSViewController alloc] init];
        self.primaryPopover.contentViewController = viewController;
    }
    viewController.view = [self primaryPanelView];
    if (viewController.view.window != nil) {
        [viewController.view.window makeFirstResponder:viewController.view];
    }
    self.needsMenuRebuild = NO;
}

- (void)popoverDidClose:(NSNotification *)notification {
    if (notification.object != self.primaryPopover) {
        return;
    }
    self.menuVisible = NO;
    self.primaryPopover = nil;
    self.needsMenuRebuild = NO;
}

- (void)menuWillOpen:(NSMenu *)menu {
    if (menu == self.mainMenu) {
        self.menuVisible = YES;
    }
}

- (void)menuDidClose:(NSMenu *)menu {
    if (menu != self.mainMenu) {
        return;
    }

    self.menuVisible = NO;
    if (self.needsMenuRebuild) {
        [self rebuildMenu];
    }
}

- (void)popUpMenu:(NSMenu *)menu fromPanel:(NSView *)view action:(NSDictionary *)action {
    NSRect frame = [action[@"frame"] rectValue];
    NSPoint location = NSMakePoint(NSMinX(frame), NSMaxY(frame));
    if (view.window == nil) {
        return;
    }
    NSRect anchorRect = NSMakeRect(location.x, location.y, 1, 1);
    NSRect windowRect = [view convertRect:anchorRect toView:nil];
    NSPoint screenLocation = [view.window convertRectToScreen:windowRect].origin;
    NSView *sourceView = view;
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.08 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        if (sourceView.window != nil) {
            [menu popUpMenuPositioningItem:nil atLocation:location inView:sourceView];
            return;
        }
        [menu popUpMenuPositioningItem:nil atLocation:screenLocation inView:nil];
    });
}

- (void)performPanelButtonAction:(id)sender {
    if (![sender isKindOfClass:OpenTamerPanelActionButton.class]) {
        return;
    }
    OpenTamerPanelActionButton *button = sender;
    NSView *sourceView = button.superview ?: button;
    [self performPanelAction:button.panelAction event:NSApp.currentEvent sourceView:sourceView];
}

- (void)performPanelAction:(NSDictionary *)action event:(NSEvent *)event sourceView:(NSView *)view {
    NSString *name = [action[@"name"] isKindOfClass:NSString.class] ? action[@"name"] : @"";

    if ([name isEqualToString:@"toggle"]) {
        [self sendCommandString:@"toggle-management"];
        return;
    }
    if ([name isEqualToString:@"refresh"]) {
        [self sendCommandString:@"refresh"];
        return;
    }
    if ([name isEqualToString:@"preferences"]) {
        [self popUpMenu:[self preferencesMenu] fromPanel:view action:action];
        return;
    }
    if ([name isEqualToString:@"all-processes"]) {
        [self popUpMenu:[self allProcessesMenu] fromPanel:view action:action];
        return;
    }
    if ([name isEqualToString:@"graph-toggle"]) {
        NSString *appKey = [action[@"appKey"] isKindOfClass:NSString.class] ? action[@"appKey"] : @"";
        [self toggleGraphLineForAppKey:appKey];
        [self refreshPrimaryPopoverContent];
        return;
    }
    if ([name isEqualToString:@"sort-current"]) {
        [self sendCommandString:@"pref-string|topProcessesSort|current"];
        return;
    }
    if ([name isEqualToString:@"sort-average"]) {
        [self sendCommandString:@"pref-string|topProcessesSort|average"];
        return;
    }
    if ([name isEqualToString:@"process"] || [name isEqualToString:@"managed"]) {
        NSDictionary *row = [action[@"row"] isKindOfClass:NSDictionary.class] ? action[@"row"] : nil;
        if (row == nil) {
            return;
        }
        NSMenu *menu = [name isEqualToString:@"managed"] ? [self managedMenuForRow:row] : [self processMenuForRow:row];
        [self popUpMenu:menu fromPanel:view action:action];
        return;
    }
}

- (void)performPanelQuit {
    [self quit:nil];
}

- (void)sendCommand:(id)sender {
    NSMenuItem *item = sender;
    NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
    [self updateMenuItem:item afterCommand:command];
    [self sendCommandString:command];
}

- (void)sendPersistentCommand:(id)sender {
    if (![sender isKindOfClass:OpenTamerCommandMenuItemView.class]) {
        return;
    }
    OpenTamerCommandMenuItemView *view = sender;
    NSMenuItem *item = view.menuItem;
    NSString *command = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
    [self updateMenuItem:item afterCommand:command];
    [self sendCommandString:command];
}

- (void)promptForCPULimit:(id)sender {
    NSMenuItem *item = sender;
    NSString *appKey = [item.representedObject isKindOfClass:NSString.class] ? item.representedObject : @"";
    if (appKey.length == 0) {
        return;
    }

    [NSApp activateIgnoringOtherApps:YES];

    NSTextField *field = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 220, 24)];
    field.stringValue = @"25";
    field.placeholderString = @"0.01";

    NSAlert *alert = [[NSAlert alloc] init];
    alert.messageText = @"Limit CPU";
    alert.informativeText = @"Enter a maximum CPU percentage. Minimum is 0.01%.";
    alert.accessoryView = field;
    [alert addButtonWithTitle:@"Apply"];
    [alert addButtonWithTitle:@"Cancel"];

    if ([alert runModal] != NSAlertFirstButtonReturn) {
        return;
    }

    NSString *rawValue = [field.stringValue stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    double value = rawValue.doubleValue;
    if (value < 0.01) {
        NSAlert *validation = [[NSAlert alloc] init];
        validation.messageText = @"CPU limit not applied";
        validation.informativeText = @"Enter a value of 0.01% or higher.";
        [validation addButtonWithTitle:@"OK"];
        [validation runModal];
        return;
    }

    NSString *command = [NSString stringWithFormat:@"rule|limit|%.6g|%@", value, appKey];
    opentamer_menu_command(command.UTF8String);
}

- (void)promptForHighCPUThreshold:(id)sender {
    [NSApp activateIgnoringOtherApps:YES];

    double current = [self floatPreference:@"highCPUThreshold" fallback:75];
    NSTextField *field = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 220, 24)];
    field.stringValue = [NSString stringWithFormat:@"%.6g", current];
    field.placeholderString = @"75";

    NSAlert *alert = [[NSAlert alloc] init];
    alert.messageText = @"High CPU Alert Threshold";
    alert.informativeText = @"Enter the system CPU percentage that should trigger alerts.";
    alert.accessoryView = field;
    [alert addButtonWithTitle:@"Apply"];
    [alert addButtonWithTitle:@"Cancel"];

    if ([alert runModal] != NSAlertFirstButtonReturn) {
        return;
    }

    NSString *rawValue = [field.stringValue stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
    double value = rawValue.doubleValue;
    if (value <= 0) {
        NSAlert *validation = [[NSAlert alloc] init];
        validation.messageText = @"Alert threshold not applied";
        validation.informativeText = @"Enter a value greater than 0%.";
        [validation addButtonWithTitle:@"OK"];
        [validation runModal];
        return;
    }

    NSString *command = [NSString stringWithFormat:@"pref-float|highCPUThreshold|%.6g", value];
    opentamer_menu_command(command.UTF8String);
}

- (void)toggleLaunchAtLogin:(id)sender {
    if (!@available(macOS 13.0, *)) {
        return;
    }

    SMAppService *service = SMAppService.mainAppService;
    NSError *error = nil;
    SMAppServiceStatus currentStatus = service.status;
    BOOL registered = currentStatus == SMAppServiceStatusEnabled ||
        currentStatus == SMAppServiceStatusRequiresApproval;
    BOOL updated = registered
        ? [service unregisterAndReturnError:&error]
        : [service registerAndReturnError:&error];
    if (!updated) {
        NSAlert *alert = [[NSAlert alloc] init];
        alert.messageText = @"Login item not updated";
        alert.informativeText = error.localizedDescription ?: @"OpenTamer could not update its Login Items setting.";
        [alert addButtonWithTitle:@"OK"];
        [alert runModal];
        return;
    }

    if ([sender isKindOfClass:NSMenuItem.class]) {
        NSMenuItem *item = (NSMenuItem *)sender;
        SMAppServiceStatus status = service.status;
        item.state = status == SMAppServiceStatusEnabled
            ? NSControlStateValueOn
            : status == SMAppServiceStatusRequiresApproval ? NSControlStateValueMixed : NSControlStateValueOff;
        item.toolTip = status == SMAppServiceStatusRequiresApproval
            ? @"Approve OpenTamer in System Settings to launch it at login."
            : nil;
    }
}

- (void)quit:(id)sender {
    opentamer_menu_command("quit");
    [NSApp terminate:nil];
}

@end

static OpenTamerMenuBarController *OpenTamerMenuBar = nil;

static NSDictionary *OpenTamerDictionaryFromJSON(const char *json) {
    if (json == NULL) {
        return @{};
    }

    NSData *data = [NSData dataWithBytes:json length:strlen(json)];
    if (data == nil) {
        return @{};
    }

    NSError *error = nil;
    id object = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];
    if (error != nil || ![object isKindOfClass:NSDictionary.class]) {
        return @{};
    }

    return object;
}

void OpenTamerUpdateMenuBarJSON(const char *state_json) {
    @autoreleasepool {
        NSDictionary *state = OpenTamerDictionaryFromJSON(state_json);
        dispatch_async(dispatch_get_main_queue(), ^{
            @autoreleasepool {
                [OpenTamerMenuBar updateWithState:state];
            }
        });
    }
}

void OpenTamerRunMenuBarApp(const char *initial_state_json) {
    @autoreleasepool {
        NSApplication *app = NSApplication.sharedApplication;
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];

        NSDictionary *state = OpenTamerDictionaryFromJSON(initial_state_json);
        OpenTamerMenuBar = [[OpenTamerMenuBarController alloc] initWithState:state];
        [OpenTamerMenuBar install];

        [app run];
    }
}

void OpenTamerTerminateMenuBarApp(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}
