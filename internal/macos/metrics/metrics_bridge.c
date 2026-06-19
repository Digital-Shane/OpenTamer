#include "metrics_bridge.h"

#include <libproc.h>
#include <mach/host_info.h>
#include <mach/mach_host.h>
#include <mach/mach_init.h>
#include <mach/mach_time.h>
#include <string.h>
#include <sys/proc_info.h>

static uint64_t OpenTamerAbsoluteToNanoseconds(uint64_t absolute_time);

int OpenTamerCopyProcessMetrics(int pid, OpenTamerProcessMetrics *out_metrics) {
    if (pid <= 0 || out_metrics == 0) {
        return 0;
    }

    struct proc_taskinfo taskinfo;
    int result = proc_pidinfo(pid, PROC_PIDTASKINFO, 0, &taskinfo, PROC_PIDTASKINFO_SIZE);
    if (result != PROC_PIDTASKINFO_SIZE) {
        return 0;
    }

    memset(out_metrics, 0, sizeof(OpenTamerProcessMetrics));
    out_metrics->pid = pid;
    out_metrics->cpu_time_ns = OpenTamerAbsoluteToNanoseconds(taskinfo.pti_total_user)
        + OpenTamerAbsoluteToNanoseconds(taskinfo.pti_total_system);
    out_metrics->resident_size = taskinfo.pti_resident_size;
    return 1;
}

static uint64_t OpenTamerAbsoluteToNanoseconds(uint64_t absolute_time) {
    static mach_timebase_info_data_t timebase = {0, 0};
    if (timebase.denom == 0) {
        kern_return_t result = mach_timebase_info(&timebase);
        if (result != KERN_SUCCESS || timebase.denom == 0) {
            return absolute_time;
        }
    }

    __uint128_t nanoseconds = (__uint128_t)absolute_time * timebase.numer / timebase.denom;
    if (nanoseconds > UINT64_MAX) {
        return UINT64_MAX;
    }
    return (uint64_t)nanoseconds;
}

int OpenTamerCopySystemCPULoad(OpenTamerSystemCPULoad *out_load) {
    if (out_load == 0) {
        return 0;
    }

    host_cpu_load_info_data_t cpuinfo;
    mach_msg_type_number_t count = HOST_CPU_LOAD_INFO_COUNT;
    kern_return_t result = host_statistics(mach_host_self(), HOST_CPU_LOAD_INFO, (host_info_t)&cpuinfo, &count);
    if (result != KERN_SUCCESS) {
        return 0;
    }

    memset(out_load, 0, sizeof(OpenTamerSystemCPULoad));
    out_load->user = cpuinfo.cpu_ticks[CPU_STATE_USER];
    out_load->nice = cpuinfo.cpu_ticks[CPU_STATE_NICE];
    out_load->system = cpuinfo.cpu_ticks[CPU_STATE_SYSTEM];
    out_load->idle = cpuinfo.cpu_ticks[CPU_STATE_IDLE];
    return 1;
}
