//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func setCredential(cmd *exec.Cmd, uid, gid int) {
	attr := &syscall.SysProcAttr{
		Credential: &syscall.Credential{},
	}
	attr.Credential.Uid = uint32(uid)
	attr.Credential.Gid = uint32(gid)
	cmd.SysProcAttr = attr
}
