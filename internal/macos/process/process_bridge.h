#ifndef OPENTAMER_PROCESS_BRIDGE_H
#define OPENTAMER_PROCESS_BRIDGE_H

#include <stdint.h>

#define OPENTAMER_PROCESS_NAME_LENGTH 64
#define OPENTAMER_PROCESS_PATH_LENGTH 4096

typedef struct {
    int pid;
    int ppid;
    int uid;
    int gid;
    int nice;
    uint64_t start_sec;
    uint64_t start_usec;
    char name[OPENTAMER_PROCESS_NAME_LENGTH];
    char path[OPENTAMER_PROCESS_PATH_LENGTH];
} OpenTamerProcessInfo;

int OpenTamerListPIDs(int *buffer, int capacity);
int OpenTamerCopyProcessInfo(int pid, OpenTamerProcessInfo *out_info);

#endif
