package supervisor

import "syscall"

// sysProcAttr puts vision_processor in its own process group, so a Ctrl-C
// in the host's terminal reaches only the host, which then stops it with
// SIGTERM; and has the kernel SIGTERM it if the host dies without doing so.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
}
