//go:build !linux

package supervisor

import "syscall"

func sysProcAttr() *syscall.SysProcAttr { return nil }
