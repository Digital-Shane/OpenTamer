#ifndef OPENTAMER_APPKIT_BRIDGE_H
#define OPENTAMER_APPKIT_BRIDGE_H

char *OpenTamerCopyRunningApplicationsJSON(void);
char *OpenTamerCopyFrontmostApplicationJSON(void);
void OpenTamerStartWorkspaceObserver(void);
int OpenTamerHideApplication(int pid);
int OpenTamerActivateApplication(int pid);
int OpenTamerTerminateApplication(int pid);

#endif
