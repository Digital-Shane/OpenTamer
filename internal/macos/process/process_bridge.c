#include "process_bridge.h"

#include <libproc.h>
#include <string.h>
#include <sys/proc_info.h>

int OpenTamerListPIDs(int *buffer, int capacity) {
    int byte_capacity = capacity <= 0 ? 0 : capacity * (int)sizeof(int);
    return proc_listpids(PROC_ALL_PIDS, 0, buffer, byte_capacity);
}

int OpenTamerCopyProcessInfo(int pid, OpenTamerProcessInfo *out_info) {
    if (pid <= 0 || out_info == 0) {
        return 0;
    }

    struct proc_bsdinfo bsdinfo;
    int result = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &bsdinfo, PROC_PIDTBSDINFO_SIZE);
    if (result != PROC_PIDTBSDINFO_SIZE) {
        return 0;
    }

    memset(out_info, 0, sizeof(OpenTamerProcessInfo));
    out_info->pid = (int)bsdinfo.pbi_pid;
    out_info->ppid = (int)bsdinfo.pbi_ppid;
    out_info->uid = (int)bsdinfo.pbi_uid;
    out_info->gid = (int)bsdinfo.pbi_gid;
    out_info->nice = (int)bsdinfo.pbi_nice;
    out_info->start_sec = bsdinfo.pbi_start_tvsec;
    out_info->start_usec = bsdinfo.pbi_start_tvusec;

    const char *name = bsdinfo.pbi_name[0] == '\0' ? bsdinfo.pbi_comm : bsdinfo.pbi_name;
    strncpy(out_info->name, name, OPENTAMER_PROCESS_NAME_LENGTH - 1);

    char path_buffer[OPENTAMER_PROCESS_PATH_LENGTH];
    memset(path_buffer, 0, sizeof(path_buffer));
    if (proc_pidpath(pid, path_buffer, sizeof(path_buffer)) > 0) {
        strncpy(out_info->path, path_buffer, OPENTAMER_PROCESS_PATH_LENGTH - 1);
    }

    return 1;
}
