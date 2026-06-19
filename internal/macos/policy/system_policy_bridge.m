#import <Cocoa/Cocoa.h>
#import <IOKit/IOKitLib.h>
#import <IOKit/hidsystem/IOHIDParameter.h>
#import <IOKit/ps/IOPowerSources.h>
#import <IOKit/ps/IOPSKeys.h>
#import <stdlib.h>
#import <string.h>

#import "system_policy_bridge.h"

static uint64_t OpenTamerLastWakeNanoseconds = 0;

static uint64_t OpenTamerCurrentUnixNanoseconds(void) {
    NSTimeInterval seconds = [[NSDate date] timeIntervalSince1970];
    if (seconds <= 0) {
        return 0;
    }
    return (uint64_t)(seconds * 1000000000.0);
}

@interface OpenTamerWakeObserver : NSObject
@end

@implementation OpenTamerWakeObserver

- (void)systemDidWake:(NSNotification *)notification {
    @synchronized ([OpenTamerWakeObserver class]) {
        OpenTamerLastWakeNanoseconds = OpenTamerCurrentUnixNanoseconds();
    }
}

@end

static OpenTamerWakeObserver *OpenTamerWakeObserverInstance = nil;

static void OpenTamerInstallWakeObserverOnMainThread(void) {
    if (OpenTamerWakeObserverInstance != nil) {
        return;
    }
    OpenTamerWakeObserverInstance = [[OpenTamerWakeObserver alloc] init];
    [NSWorkspace.sharedWorkspace.notificationCenter addObserver:OpenTamerWakeObserverInstance
                                                       selector:@selector(systemDidWake:)
                                                           name:NSWorkspaceDidWakeNotification
                                                         object:nil];
}

static void OpenTamerInstallWakeObserver(void) {
    if ([NSThread isMainThread]) {
        OpenTamerInstallWakeObserverOnMainThread();
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        OpenTamerInstallWakeObserverOnMainThread();
    });
}

static uint64_t OpenTamerCopyLastWakeNanoseconds(void) {
    @synchronized ([OpenTamerWakeObserver class]) {
        return OpenTamerLastWakeNanoseconds;
    }
}

static void OpenTamerFillPowerState(NSMutableDictionary *state) {
    CFTypeRef info = IOPSCopyPowerSourcesInfo();
    if (info == NULL) {
        return;
    }

    CFArrayRef sources = IOPSCopyPowerSourcesList(info);
    if (sources == NULL) {
        CFRelease(info);
        return;
    }

    BOOL onACPower = NO;
    NSNumber *batteryPercent = nil;

    CFIndex count = CFArrayGetCount(sources);
    for (CFIndex i = 0; i < count; i++) {
        CFTypeRef source = CFArrayGetValueAtIndex(sources, i);
        NSDictionary *description = (__bridge NSDictionary *)IOPSGetPowerSourceDescription(info, source);
        if (description == nil) {
            continue;
        }

        NSString *powerState = [description objectForKey:@(kIOPSPowerSourceStateKey)];
        if ([powerState isEqualToString:@(kIOPSACPowerValue)]) {
            onACPower = YES;
        }

        NSNumber *current = [description objectForKey:@(kIOPSCurrentCapacityKey)];
        NSNumber *maximum = [description objectForKey:@(kIOPSMaxCapacityKey)];
        if (current != nil && maximum != nil && maximum.doubleValue > 0) {
            batteryPercent = @((current.doubleValue / maximum.doubleValue) * 100.0);
        }
    }

    state[@"onACPower"] = @(onACPower);
    if (batteryPercent != nil) {
        state[@"batteryPercent"] = batteryPercent;
    }

    CFRelease(sources);
    CFRelease(info);
}

static uint64_t OpenTamerCopyIdleNanoseconds(void) {
    io_registry_entry_t entry = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching("IOHIDSystem"));
    if (entry == MACH_PORT_NULL) {
        return 0;
    }

    CFMutableDictionaryRef properties = NULL;
    kern_return_t result = IORegistryEntryCreateCFProperties(entry, &properties, kCFAllocatorDefault, 0);
    IOObjectRelease(entry);
    if (result != KERN_SUCCESS || properties == NULL) {
        return 0;
    }

    NSDictionary *dictionary = (__bridge NSDictionary *)properties;
    NSNumber *idle = [dictionary objectForKey:@(kIOHIDIdleTimeKey)];
    uint64_t nanoseconds = idle == nil ? 0 : idle.unsignedLongLongValue;
    CFRelease(properties);
    return nanoseconds;
}

char *OpenTamerCopySystemPolicyJSON(void) {
    @autoreleasepool {
        OpenTamerInstallWakeObserver();
        NSMutableDictionary *state = [NSMutableDictionary dictionary];
        state[@"onACPower"] = @NO;
        state[@"userIdleNanoseconds"] = @(OpenTamerCopyIdleNanoseconds());
        uint64_t lastWake = OpenTamerCopyLastWakeNanoseconds();
        if (lastWake > 0) {
            state[@"lastWakeUnixNanoseconds"] = @(lastWake);
        }
        OpenTamerFillPowerState(state);

        NSError *error = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:state options:0 error:&error];
        if (data == nil || error != nil) {
            return strdup("{}");
        }
        NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        if (json == nil) {
            return strdup("{}");
        }
        return strdup(json.UTF8String);
    }
}
