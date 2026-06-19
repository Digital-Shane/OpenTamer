#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>
#import <stdlib.h>
#import <string.h>

#import "notifications_bridge.h"

@interface OpenTamerNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
+ (instancetype)sharedDelegate;
@end

@implementation OpenTamerNotificationDelegate

+ (instancetype)sharedDelegate {
    static OpenTamerNotificationDelegate *delegate = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        delegate = [[OpenTamerNotificationDelegate alloc] init];
    });
    return delegate;
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
         willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions options))completionHandler {
    UNNotificationPresentationOptions options = UNNotificationPresentationOptionSound |
        UNNotificationPresentationOptionBanner |
        UNNotificationPresentationOptionList;
    completionHandler(options);
}

@end

static char *OpenTamerCopyCString(NSString *message) {
    if (message == nil) {
        return strdup("OpenTamer notification failed");
    }
    const char *utf8 = [message UTF8String];
    if (utf8 == NULL) {
        return strdup("OpenTamer notification failed");
    }
    return strdup(utf8);
}

static char *OpenTamerCopyNSError(NSString *prefix, NSError *error) {
    if (error == nil) {
        return OpenTamerCopyCString(prefix);
    }
    NSString *details = error.localizedDescription ?: error.description;
    return OpenTamerCopyCString([NSString stringWithFormat:@"%@: %@", prefix, details]);
}

static void OpenTamerAssignError(char **error_out, char *message) {
    if (message == NULL) {
        return;
    }
    if (error_out != NULL && *error_out == NULL) {
        *error_out = message;
        return;
    }
    free(message);
}

static int OpenTamerNotificationAuthorized(UNUserNotificationCenter *center, char **error_out) {
    __block BOOL authorized = NO;
    __block char *authorizationError = NULL;
    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);

    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        switch (settings.authorizationStatus) {
            case UNAuthorizationStatusAuthorized:
            case UNAuthorizationStatusProvisional:
                authorized = YES;
                dispatch_semaphore_signal(semaphore);
                break;
            case UNAuthorizationStatusNotDetermined: {
                [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                                      completionHandler:^(BOOL granted, NSError * _Nullable error) {
                    authorized = granted && error == nil;
                    if (error != nil) {
                        authorizationError = OpenTamerCopyNSError(@"OpenTamer notification authorization failed", error);
                    } else if (!granted) {
                        authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization was not granted");
                    }
                    dispatch_semaphore_signal(semaphore);
                }];
                break;
            }
            case UNAuthorizationStatusDenied:
                authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization denied");
                dispatch_semaphore_signal(semaphore);
                break;
            default:
                authorizationError = OpenTamerCopyCString(@"OpenTamer notification authorization unavailable");
                dispatch_semaphore_signal(semaphore);
                break;
        }
    }];

    long result = dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 30 * NSEC_PER_SEC));
    if (result != 0) {
        free(authorizationError);
        OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification authorization timed out"));
        return 0;
    }
    if (!authorized) {
        OpenTamerAssignError(error_out, authorizationError != NULL ? authorizationError : OpenTamerCopyCString(@"OpenTamer notification authorization failed"));
        return 0;
    }
    free(authorizationError);
    return 1;
}

int OpenTamerDeliverNotification(const char *title, const char *body, char **error_out) {
    @autoreleasepool {
        if (error_out != NULL) {
            *error_out = NULL;
        }
        NSString *titleString = title == NULL ? @"OpenTamer" : [NSString stringWithUTF8String:title];
        NSString *bodyString = body == NULL ? @"" : [NSString stringWithUTF8String:body];
        if (titleString == nil || bodyString == nil) {
            OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification text is not valid UTF-8"));
            return 0;
        }

        UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
        center.delegate = [OpenTamerNotificationDelegate sharedDelegate];
        if (OpenTamerNotificationAuthorized(center, error_out) != 1) {
            return 0;
        }

        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = titleString;
        content.body = bodyString;

        NSString *identifier = [NSString stringWithFormat:@"opentamer-high-cpu-%@", NSUUID.UUID.UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                              content:content
                                                                              trigger:nil];

        __block BOOL delivered = NO;
        __block char *deliveryError = NULL;
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
        [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
            delivered = error == nil;
            if (error != nil) {
                deliveryError = OpenTamerCopyNSError(@"OpenTamer notification delivery failed", error);
            }
            dispatch_semaphore_signal(semaphore);
        }];

        long result = dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
        if (result != 0) {
            free(deliveryError);
            OpenTamerAssignError(error_out, OpenTamerCopyCString(@"OpenTamer notification delivery timed out"));
            return 0;
        }
        if (!delivered) {
            OpenTamerAssignError(error_out, deliveryError != NULL ? deliveryError : OpenTamerCopyCString(@"OpenTamer notification delivery failed"));
            return 0;
        }
        free(deliveryError);
        return 1;
    }
}
