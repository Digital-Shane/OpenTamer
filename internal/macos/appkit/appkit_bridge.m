#import <Cocoa/Cocoa.h>
#import <stdlib.h>
#import <string.h>

#import "appkit_bridge.h"

extern void opentamer_app_event(const char *payload);

static NSDictionary *OpenTamerAppDictionary(NSRunningApplication *app) {
    if (app == nil) {
        return @{};
    }

    NSString *bundleID = app.bundleIdentifier ?: @"";
    NSString *localizedName = app.localizedName ?: @"";
    NSString *executablePath = app.executableURL.path ?: @"";
    NSString *bundlePath = app.bundleURL.path ?: @"";

    return @{
        @"pid": @(app.processIdentifier),
        @"bundleID": bundleID,
        @"localizedName": localizedName,
        @"executablePath": executablePath,
        @"bundlePath": bundlePath,
        @"activationPolicy": @(app.activationPolicy),
        @"active": @(app.active),
        @"hidden": @(app.hidden),
        @"terminated": @(app.terminated)
    };
}

static char *OpenTamerCopyJSONObject(id object) {
    @autoreleasepool {
        if (object == nil) {
            object = @{};
        }

        NSError *error = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:object options:0 error:&error];
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

char *OpenTamerCopyRunningApplicationsJSON(void) {
    @autoreleasepool {
        NSArray<NSRunningApplication *> *running = NSWorkspace.sharedWorkspace.runningApplications;
        NSMutableArray *apps = [NSMutableArray arrayWithCapacity:running.count];

        for (NSRunningApplication *app in running) {
            [apps addObject:OpenTamerAppDictionary(app)];
        }

        return OpenTamerCopyJSONObject(apps);
    }
}

char *OpenTamerCopyFrontmostApplicationJSON(void) {
    @autoreleasepool {
        NSRunningApplication *app = NSWorkspace.sharedWorkspace.frontmostApplication;
        return OpenTamerCopyJSONObject(OpenTamerAppDictionary(app));
    }
}

static NSRunningApplication *OpenTamerApplicationForPID(int pid) {
    if (pid <= 0) {
        return nil;
    }
    return [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
}

int OpenTamerHideApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *app = OpenTamerApplicationForPID(pid);
        if (app == nil) {
            return 0;
        }
        return [app hide] ? 1 : 0;
    }
}

int OpenTamerActivateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *app = OpenTamerApplicationForPID(pid);
        if (app == nil) {
            return 0;
        }
        return [app activateWithOptions:NSApplicationActivateAllWindows] ? 1 : 0;
    }
}

int OpenTamerTerminateApplication(int pid) {
    @autoreleasepool {
        NSRunningApplication *app = OpenTamerApplicationForPID(pid);
        if (app == nil) {
            return 0;
        }
        return [app terminate] ? 1 : 0;
    }
}

@interface OpenTamerWorkspaceObserver : NSObject
@end

@implementation OpenTamerWorkspaceObserver

- (void)emitKind:(NSString *)kind notification:(NSNotification *)notification {
    NSRunningApplication *app = notification.userInfo[NSWorkspaceApplicationKey];
    if (app == nil) {
        return;
    }

    NSMutableDictionary *event = [NSMutableDictionary dictionary];
    event[@"kind"] = kind;
    event[@"app"] = OpenTamerAppDictionary(app);

    char *payload = OpenTamerCopyJSONObject(event);
    opentamer_app_event(payload);
    free(payload);
}

- (void)applicationDidLaunch:(NSNotification *)notification {
    [self emitKind:@"launched" notification:notification];
}

- (void)applicationDidTerminate:(NSNotification *)notification {
    [self emitKind:@"terminated" notification:notification];
}

- (void)applicationDidActivate:(NSNotification *)notification {
    [self emitKind:@"activated" notification:notification];
}

- (void)applicationDidHide:(NSNotification *)notification {
    [self emitKind:@"hidden" notification:notification];
}

- (void)applicationDidUnhide:(NSNotification *)notification {
    [self emitKind:@"unhidden" notification:notification];
}

@end

static OpenTamerWorkspaceObserver *OpenTamerObserver = nil;

static void OpenTamerInstallWorkspaceObserver(void) {
    if (OpenTamerObserver != nil) {
        return;
    }

    OpenTamerObserver = [[OpenTamerWorkspaceObserver alloc] init];
    NSNotificationCenter *center = NSWorkspace.sharedWorkspace.notificationCenter;

    [center addObserver:OpenTamerObserver
               selector:@selector(applicationDidLaunch:)
                   name:NSWorkspaceDidLaunchApplicationNotification
                 object:nil];
    [center addObserver:OpenTamerObserver
               selector:@selector(applicationDidTerminate:)
                   name:NSWorkspaceDidTerminateApplicationNotification
                 object:nil];
    [center addObserver:OpenTamerObserver
               selector:@selector(applicationDidActivate:)
                   name:NSWorkspaceDidActivateApplicationNotification
                 object:nil];
    [center addObserver:OpenTamerObserver
               selector:@selector(applicationDidHide:)
                   name:NSWorkspaceDidHideApplicationNotification
                 object:nil];
    [center addObserver:OpenTamerObserver
               selector:@selector(applicationDidUnhide:)
                   name:NSWorkspaceDidUnhideApplicationNotification
                 object:nil];
}

void OpenTamerStartWorkspaceObserver(void) {
    if ([NSThread isMainThread]) {
        OpenTamerInstallWorkspaceObserver();
        return;
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        OpenTamerInstallWorkspaceObserver();
    });
}
