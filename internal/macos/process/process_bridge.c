#include "process_bridge.h"

#include <errno.h>
#include <libproc.h>
#include <string.h>
#include <sys/proc_info.h>

int OpenTamerListPIDs(int *buffer, int capacity) {
    int byte_capacity = capacity <= 0 ? 0 : capacity * (int)sizeof(int);
    return proc_listpids(PROC_ALL_PIDS, 0, buffer, byte_capacity);
}

static int copy_bsdinfo(int pid, struct proc_bsdinfo *info) {
    errno = 0;
    int result = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, info, PROC_PIDTBSDINFO_SIZE);
    if (result != PROC_PIDTBSDINFO_SIZE) {
        return -(result <= 0 && errno != 0 ? errno : EIO);
    }
    return info->pbi_pid == (uint32_t)pid ? 1 : -EIO;
}

int OpenTamerCopyProcessInfo(int pid, OpenTamerProcessInfo *out_info) {
    if (out_info == 0) {
        return -EINVAL;
    }
    memset(out_info, 0, sizeof(OpenTamerProcessInfo));
    if (pid <= 0) {
        return -EINVAL;
    }

    struct proc_bsdinfo bsdinfo;
    int result = copy_bsdinfo(pid, &bsdinfo);
    if (result != 1) {
        return result;
    }

    char path_buffer[OPENTAMER_PROCESS_PATH_LENGTH];
    memset(path_buffer, 0, sizeof(path_buffer));
    if (proc_pidpath(pid, path_buffer, sizeof(path_buffer)) <= 0) {
        path_buffer[0] = '\0';
    }

    struct proc_bsdinfo current;
    result = copy_bsdinfo(pid, &current);
    if (result != 1) {
        return result;
    }
    if (bsdinfo.pbi_start_tvsec != current.pbi_start_tvsec ||
        bsdinfo.pbi_start_tvusec != current.pbi_start_tvusec ||
        bsdinfo.pbi_uid != current.pbi_uid || bsdinfo.pbi_gid != current.pbi_gid ||
        strncmp(bsdinfo.pbi_name, current.pbi_name, sizeof(bsdinfo.pbi_name)) != 0 ||
        strncmp(bsdinfo.pbi_comm, current.pbi_comm, sizeof(bsdinfo.pbi_comm)) != 0) {
        return -EAGAIN;
    }
    // Same-name exec can still change the path within a process generation.
    out_info->pid = (int)bsdinfo.pbi_pid;
    out_info->ppid = (int)bsdinfo.pbi_ppid;
    out_info->uid = (int)bsdinfo.pbi_uid;
    out_info->gid = (int)bsdinfo.pbi_gid;
    out_info->nice = (int)bsdinfo.pbi_nice;
    out_info->start_sec = bsdinfo.pbi_start_tvsec;
    out_info->start_usec = bsdinfo.pbi_start_tvusec;

    const char *name = bsdinfo.pbi_name[0] == '\0' ? bsdinfo.pbi_comm : bsdinfo.pbi_name;
    strncpy(out_info->name, name, OPENTAMER_PROCESS_NAME_LENGTH - 1);

    strncpy(out_info->path, path_buffer, OPENTAMER_PROCESS_PATH_LENGTH - 1);

    return 1;
}
