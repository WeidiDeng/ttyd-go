//go:build !windows

package main

import (
	"flag"
	"log"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

var (
	uidSet bool
	gidSet bool

	trueUid int
	trueGid int
)

func init() {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "uid":
			uidSet = true
		case "gid":
			gidSet = true
		}
	})
	if uidSet || gidSet {
		u, err := user.Current()
		if err != nil {
			log.Fatalln("failed to get current user:", err)
		}
		userUid, err := strconv.Atoi(u.Uid)
		if err != nil {
			log.Fatalln("failed to parse current user id:", err)
		}
		userGid, err := strconv.Atoi(u.Gid)
		if err != nil {
			log.Fatalln("failed to parse current group id:", err)
		}

		if uidSet {
			trueUid = *uid
		} else {
			trueUid = userUid
		}

		if gidSet {
			trueGid = *gid
		} else {
			trueGid = userGid
		}
	}

}

func setCredential(cmd *exec.Cmd) {
	if uidSet || gidSet {
		attr := &syscall.SysProcAttr{
			Credential: &syscall.Credential{},
		}
		attr.Credential.Uid = uint32(trueUid)
		attr.Credential.Gid = uint32(trueGid)
		cmd.SysProcAttr = attr
	}
}
