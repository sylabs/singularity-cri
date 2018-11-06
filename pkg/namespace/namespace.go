// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			procFile:  "pid",
		},
		specs.NetworkNamespace: {
			cloneFlag: syscall.CLONE_NEWNET,
			procFile:  "net",
		},
		specs.MountNamespace: {
			cloneFlag: syscall.CLONE_NEWNS,
			procFile:  "mnt",
		},
		specs.IPCNamespace: {
			cloneFlag: syscall.CLONE_NEWIPC,
			procFile:  "ipc",
		},
		specs.UTSNamespace: {
			cloneFlag: syscall.CLONE_NEWUTS,
			procFile:  "uts",
		},
		specs.UserNamespace: {
			cloneFlag: syscall.CLONE_NEWUSER,
			procFile:  "user",
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
		if err := Bind(cmd.Process.Pid, ns); err != nil {
			return fmt.Errorf("could not bind namespace: %v", err)
		}
	}
	stdin.Close()
	return nil
}

// Remove unmounts and removes namespace file at ns.Path. Remove doesn't
// return an error if namespace is not mounted or file doesn't exist.
func Remove(ns specs.LinuxNamespace) error {
	err := syscall.Unmount(ns.Path, syscall.MNT_DETACH)
	if err != nil && err != syscall.ENOENT && err != syscall.EINVAL {
		return fmt.Errorf("could not umount: %v", err)
	}
	err = os.Remove(ns.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove %s: %v", ns.Path, err)
	}
	return nil
}

// Bind creates namespace file at ns.Path and mounts corresponding
// namespace of process with passed pid to it with syscall.MS_BIND flag.
func Bind(pid int, ns specs.LinuxNamespace) error {
	f, err := os.Create(ns.Path)
	if err != nil {
		return fmt.Errorf("could not create %s: %v", ns.Path, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("could not close %s: %v", ns.Path, err)
	}
	source := fmt.Sprintf("/proc/%d/ns/%s", pid, nsToInfo[ns.Type].procFile)
	err = syscall.Mount(source, ns.Path, "", syscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("could not mount %s: %v", source, err)
	}
	return nil
}
