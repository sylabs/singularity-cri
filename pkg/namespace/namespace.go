package namespace

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

type (
	nsInfo struct {
		cloneFlag int
		procFile  string
	}
)

var (
	nsToInfo = map[specs.LinuxNamespaceType]nsInfo{
		specs.PIDNamespace: {
			cloneFlag: syscall.CLONE_NEWPID,
			procFile:  "",
		},
		specs.NetworkNamespace: {
			cloneFlag: syscall.CLONE_NEWNET,
			procFile:  "",
		},
		specs.MountNamespace: {
			cloneFlag: syscall.CLONE_NEWNS,
			procFile:  "",
		},
		specs.IPCNamespace: {
			cloneFlag: syscall.CLONE_NEWIPC,
			procFile:  "",
		},
		specs.UTSNamespace: {
			cloneFlag: syscall.CLONE_NEWUTS,
			procFile:  "",
		},
		specs.UserNamespace: {
			cloneFlag: syscall.CLONE_NEWUSER,
			procFile:  "",
		},
	}
)

// UnshareAll is used to create passed namespaces and save them
// for the later use. After call to UnshareAll passed namespaces
// can be found at LinuxNamespace.Path.
func UnshareAll(namespaces []specs.LinuxNamespace) error {
	if len(namespaces) == 0 {
		return nil
	}

	var cloneFlags int
	for _, ns := range namespaces {
		cloneFlags |= nsToInfo[ns.Type].cloneFlag
	}
	cmd := exec.Command("/bin/sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(cloneFlags),
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not connect to stdin: %v", err)
	}
	defer stdin.Close()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start process: %v", err)
	}
	defer cmd.Wait()

	for _, ns := range namespaces {
		if err := bindNamespace(cmd.Process.Pid, ns); err != nil {
			return fmt.Errorf("could not bind namespace: %v", err)
		}
	}
	stdin.Close()
	return nil
}

// Remove unmounts and removes namespace file at ns.Path.
func Remove(ns specs.LinuxNamespace) error {
	if err := syscall.Unmount(ns.Path, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("could not umount: %v", err)
	}
	if err := os.Remove(ns.Path); err != nil {
		return fmt.Errorf("could not remove %s: %v", ns.Path, err)
	}
	return nil
}

// bindNamespace creates namespace file at ns.Path and mounts corresponding
// namespace of process with passed pid to it with syscall.MS_BIND flag.
func bindNamespace(pid int, ns specs.LinuxNamespace) error {
	f, err := os.Create(ns.Path)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", ns.Path, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", ns.Path, err)
	}
	err = syscall.Mount(fmt.Sprintf("/proc/%d/ns/%s", pid, nsToInfo[ns.Type].procFile), ns.Path, "", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("could not mount: %v", err)
	}
	return nil
}
