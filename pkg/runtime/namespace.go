package runtime

import (
	"fmt"
	"os"
	"syscall"

	"os/exec"

	"github.com/opencontainers/runtime-spec/specs-go"
)

var (
	nsTypeToSyscallFlag = map[specs.LinuxNamespaceType]int{
		specs.PIDNamespace:     syscall.CLONE_NEWPID,
		specs.NetworkNamespace: syscall.CLONE_NEWNET,
		specs.MountNamespace:   syscall.CLONE_NEWNS,
		specs.IPCNamespace:     syscall.CLONE_NEWIPC,
		specs.UTSNamespace:     syscall.CLONE_NEWUTS,
		specs.UserNamespace:    syscall.CLONE_NEWUSER,
	}
	nsTypeToProcFile = map[specs.LinuxNamespaceType]string{
		specs.PIDNamespace:     "pid",
		specs.NetworkNamespace: "net",
		specs.MountNamespace:   "mnt",
		specs.IPCNamespace:     "ipc",
		specs.UTSNamespace:     "uts",
		specs.UserNamespace:    "user",
	}
)

// unshareNamespaces is used to create passed namespaces and save them
// for the later use. Passed namespaces may be later found at LinuxNamespace.Path.
func unshareNamespaces(namespaces []specs.LinuxNamespace) error {
	if len(namespaces) == 0 {
		return nil
	}

	var cloneFlags int
	for _, ns := range namespaces {
		cloneFlags |= nsTypeToSyscallFlag[ns.Type]
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
	err = syscall.Mount(fmt.Sprintf("/proc/%d/ns/%s", pid, nsTypeToProcFile[ns.Type]), ns.Path, "", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("could not mount: %v", err)
	}
	return nil
}

// removeNamespace unmounts and removes namespace file as ns.Path.
func removeNamespace(ns specs.LinuxNamespace) error {
	if err := syscall.Unmount(ns.Path, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("could not umount: %v", err)
	}
	if err := os.Remove(ns.Path); err != nil {
		return fmt.Errorf("could not remove %s: %v", ns.Path, err)
	}
	return nil
}
