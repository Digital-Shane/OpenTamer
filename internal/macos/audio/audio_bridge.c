#include "audio_bridge.h"

#include <CoreAudio/CoreAudio.h>

int OpenTamerAudioOutputIsRunning(void) {
    AudioObjectID device = kAudioObjectUnknown;
    UInt32 size = sizeof(device);
    AudioObjectPropertyAddress default_output = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };

    OSStatus status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &default_output, 0, NULL, &size, &device);
    if (status != noErr || device == kAudioObjectUnknown) {
        return -1;
    }

    UInt32 running = 0;
    size = sizeof(running);
    AudioObjectPropertyAddress running_property = {
        kAudioDevicePropertyDeviceIsRunningSomewhere,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };

    status = AudioObjectGetPropertyData(device, &running_property, 0, NULL, &size, &running);
    if (status != noErr) {
        return -1;
    }
    return running == 0 ? 0 : 1;
}
