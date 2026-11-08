#include <assert.h>
#include <errno.h>
#include <libproc.h>
#include <string.h>
#include <sys/proc_info.h>

static int calls;
static int failure_call;
static int failure_errno;
static int failure_size;
static int path_errno;
static int changed_generation;
static int wrong_pid;
static int fallback_name;
static int changed_metadata;

static int test_pidinfo(int pid, int flavor, uint64_t arg, void *buffer, int size) {
    assert(pid == 42);
    assert(flavor == PROC_PIDTBSDINFO);
    assert(arg == 0);
    assert(size == PROC_PIDTBSDINFO_SIZE);
    calls++;
    if (calls == failure_call) {
        errno = failure_errno;
        return failure_size;
    }
    struct proc_bsdinfo *info = buffer;
    memset(info, 0, sizeof(*info));
    info->pbi_pid = wrong_pid == calls ? 43 : 42;
    info->pbi_ppid = 12;
    info->pbi_uid = 501;
    info->pbi_gid = 20;
    info->pbi_nice = calls;
    info->pbi_start_tvsec = calls == 2 && changed_generation == 2 ? 101 : 100;
    info->pbi_start_tvusec = calls == 2 && changed_generation ? 8 : 7;
    if (!fallback_name) {
        strcpy(info->pbi_name, "child");
    }
    strcpy(info->pbi_comm, "fallback");
    if (calls == 2) {
        switch (changed_metadata) {
        case 1: info->pbi_uid++; break;
        case 2: info->pbi_gid++; break;
        case 3: strcpy(info->pbi_name, "changed"); break;
        case 4: strcpy(info->pbi_comm, "changed"); break;
        }
    }
    return size;
}

static int test_pidpath(int pid, void *buffer, uint32_t size) {
    assert(pid == 42);
    assert(calls == 1);
    assert(size >= sizeof("/bin/child"));
    strcpy(buffer, "/bin/child");
    errno = path_errno;
    return path_errno ? 0 : (int)strlen(buffer);
}

// Compile the production bridge against deterministic libproc substitutes.
#define proc_pidinfo test_pidinfo
#define proc_pidpath test_pidpath
#include "../process_bridge.c"

static void reset(void) {
    calls = 0;
    failure_call = 0;
    failure_errno = 0;
    failure_size = 0;
    path_errno = 0;
    changed_generation = 0;
    wrong_pid = 0;
    fallback_name = 0;
    changed_metadata = 0;
}

static void expect_failure(int expected) {
    OpenTamerProcessInfo info;
    memset(&info, 0xff, sizeof(info));
    assert(OpenTamerCopyProcessInfo(42, &info) == -expected);
    OpenTamerProcessInfo empty = {0};
    assert(memcmp(&info, &empty, sizeof(info)) == 0);
}

int main(void) {
    OpenTamerProcessInfo info;
    assert(OpenTamerCopyProcessInfo(42, NULL) == -EINVAL);
    assert(OpenTamerCopyProcessInfo(0, &info) == -EINVAL);
    assert(OpenTamerCopyProcessInfo(-1, &info) == -EINVAL);

    reset();
    errno = EPERM;
    assert(OpenTamerCopyProcessInfo(42, &info) == 1);
    assert(calls == 2);
    assert(info.pid == 42 && info.ppid == 12);
    assert(info.uid == 501 && info.gid == 20 && info.nice == 1);
    assert(info.start_sec == 100 && info.start_usec == 7);
    assert(strcmp(info.name, "child") == 0);
    assert(strcmp(info.path, "/bin/child") == 0);

    int errors[] = {ESRCH, ENOENT, EPERM, EACCES, EIO, EINTR, EAGAIN};
    for (int pass = 1; pass <= 2; pass++) {
        for (unsigned int i = 0; i < sizeof(errors) / sizeof(errors[0]); i++) {
            reset();
            failure_call = pass;
            failure_errno = errors[i];
            expect_failure(errors[i]);
            assert(calls == pass);

            reset();
            failure_call = pass;
            failure_errno = errors[i];
            failure_size = -1;
            expect_failure(errors[i]);
        }
        reset();
        failure_call = pass;
        errno = ESRCH;
        path_errno = ESRCH;
        expect_failure(EIO);

        reset();
        failure_call = pass;
        failure_errno = ESRCH;
        failure_size = PROC_PIDTBSDINFO_SIZE - 1;
        expect_failure(EIO);
    }

    for (int generation = 1; generation <= 2; generation++) {
        reset();
        changed_generation = generation;
        expect_failure(EAGAIN);
    }

    for (int field = 1; field <= 4; field++) {
        reset();
        changed_metadata = field;
        expect_failure(EAGAIN);
    }

    for (int pass = 1; pass <= 2; pass++) {
        reset();
        wrong_pid = pass;
        expect_failure(EIO);
    }

    reset();
    path_errno = EPERM;
    assert(OpenTamerCopyProcessInfo(42, &info) == 1);
    assert(calls == 2 && info.path[0] == '\0');
    assert(info.pid == 42 && info.start_sec == 100 && info.start_usec == 7);

    reset();
    fallback_name = 1;
    assert(OpenTamerCopyProcessInfo(42, &info) == 1);
    assert(strcmp(info.name, "fallback") == 0);
    return 0;
}
