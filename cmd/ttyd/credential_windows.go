//go:build !windows

package main

import "os/exec"

func setCredential(cmd *exec.Cmd, uid, gid int) {
}
