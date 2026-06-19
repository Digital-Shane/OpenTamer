#ifndef OPENTAMER_METRICS_BRIDGE_H
#define OPENTAMER_METRICS_BRIDGE_H

#include <stdint.h>

typedef struct {
    int pid;
    uint64_t cpu_time_ns;
    uint64_t resident_size;
} OpenTamerProcessMetrics;

typedef struct {
    uint64_t user;
    uint64_t nice;
    uint64_t system;
    uint64_t idle;
} OpenTamerSystemCPULoad;

int OpenTamerCopyProcessMetrics(int pid, OpenTamerProcessMetrics *out_metrics);
int OpenTamerCopySystemCPULoad(OpenTamerSystemCPULoad *out_load);

#endif
