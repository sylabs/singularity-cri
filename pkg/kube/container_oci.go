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

package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"golang.org/x/sys/unix"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type containerTranslator struct {
	cont *Container
	pod  *Pod
	g    generate.Generator
}

// translateContainer translates Container and its parent Pod instances
// into OCI container specification.
func translateContainer(cont *Container, pod *Pod) (*specs.Spec, error) {
	g, err := generate.New("linux")
	if err != nil {
		return nil, fmt.Errorf("could not initialize generator: %v", err)
	}
	t := containerTranslator{
		g:    g,
		cont: cont,
		pod:  pod,
	}
	return t.translate()
}

func (t *containerTranslator) translate() (*specs.Spec, error) {
	t.configureImage()
	if err := t.configureDevices(); err != nil {
		return nil, fmt.Errorf("could not configure devices: %v", err)
	}
	if err := t.configureMounts(); err != nil {
		return nil, fmt.Errorf("could not configure mounts: %v", err)
	}
	if err := t.configureProcess(); err != nil {
		return nil, fmt.Errorf("could not configure container process: %v", err)
	}
	t.configureNamespaces()
	t.configureResources()
	t.configureAnnotations()
	return t.g.Config, nil
}

func (t *containerTranslator) configureImage() {
	t.g.SetRootPath(t.cont.rootfsPath())
	t.g.SetRootReadonly(t.cont.GetLinux().GetSecurityContext().GetReadonlyRootfs())
}

func (t *containerTranslator) configureMounts() error {
	if err := t.g.SetLinuxRootPropagation("rshared"); err != nil {
		return fmt.Errorf("could not set root propagation: %v", err)
	}

	if t.pod.GetDnsConfig() != nil {
		t.g.AddMount(specs.Mount{
			Destination: "/etc/resolv.conf",
			Source:      t.pod.resolvConfFilePath(),
			Options:     []string{"bind", "ro"},
		})
	}
	t.g.SetHostname(t.pod.GetHostname())
	t.g.AddMount(specs.Mount{
		Destination: "/etc/hostname",
		Source:      t.pod.hostnameFilePath(),
		Options:     []string{"bind", "ro"},
	})

	if t.cont.GetLinux().GetSecurityContext().GetPrivileged() {
		mounts := t.g.Mounts()
		for i := range mounts {
			switch mounts[i].Type {
			case "sysfs", "procfs", "proc":
				mounts[i].Options = []string{"nosuid", "noexec", "nodev", "rw"}
			}
		}
	}

	for _, mount := range t.cont.GetMounts() {
		source, err := filepath.EvalSymlinks(mount.GetHostPath())
		if err != nil {
			return fmt.Errorf("invalid bind mount source: %v", err)
		}

		volume := specs.Mount{
			Source:      source,
			Destination: mount.GetContainerPath(),
			Options:     []string{"rbind"},
		}
		if mount.GetReadonly() {
			volume.Options = append(volume.Options, "ro")
		}
		switch mount.GetPropagation() {
		case k8s.MountPropagation_PROPAGATION_PRIVATE:
			volume.Options = append(volume.Options, "rprivate")
		case k8s.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			volume.Options = append(volume.Options, "rslave")
		case k8s.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			volume.Options = append(volume.Options, "rshared")
		}
		t.g.AddMount(volume)
	}

	return nil
}

func (t *containerTranslator) configureDevices() error {
	if t.cont.GetLinux().GetSecurityContext().GetPrivileged() {
		t.g.AddMount(specs.Mount{
			Destination: "/dev",
			Source:      "/dev",
			Options:     []string{"rbind", "nosuid", "noexec"},
		})
		t.g.Config.Linux.Resources.Devices = []specs.LinuxDeviceCgroup{{Allow: true, Access: "rwm"}}
		return nil
	}

	for _, dev := range t.cont.GetDevices() {
		device, err := t.device(dev.GetHostPath(), dev.GetContainerPath())
		if err != nil {
			return err
		}
		t.g.AddDevice(*device)
		t.g.AddLinuxResourcesDevice(true, device.Type, &device.Major, &device.Minor, dev.GetPermissions())
	}
	return nil
}

func (t *containerTranslator) device(from, to string) (*specs.LinuxDevice, error) {
	stat, err := os.Stat(from)
	if err != nil {
		return nil, fmt.Errorf("invalid device source: %v", err)
	}
	sys := stat.Sys().(*syscall.Stat_t)

	mode := stat.Mode()
	var devType string
	if mode&syscall.S_IFBLK == syscall.S_IFBLK {
		devType = "b"
	}
	if mode&syscall.S_IFCHR == syscall.S_IFCHR {
		devType = "c"
	}
	if devType == "" {
		return nil, fmt.Errorf("unsupported device type")
	}
	major := int64(unix.Major(sys.Rdev))
	minor := int64(unix.Minor(sys.Rdev))

	return &specs.LinuxDevice{
		Path:     to,
		Type:     devType,
		Major:    major,
		Minor:    minor,
		FileMode: &mode,
		UID:      &sys.Uid,
		GID:      &sys.Gid,
	}, nil
}

func (t *containerTranslator) configureNamespaces() {
	t.g.ClearLinuxNamespaces()
	t.g.AddOrReplaceLinuxNamespace(specs.UTSNamespace, t.pod.namespacePath(specs.UTSNamespace))
	t.g.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")

	security := t.cont.GetLinux().GetSecurityContext()
	switch security.GetNamespaceOptions().GetIpc() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, "")
	case k8s.NamespaceMode_POD:
		podNsPath := t.pod.namespacePath(specs.IPCNamespace)
		if podNsPath != "" {
			t.g.AddOrReplaceLinuxNamespace(specs.IPCNamespace, podNsPath)
		}
	}
	switch security.GetNamespaceOptions().GetNetwork() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, "")
	case k8s.NamespaceMode_POD:
		podNsPath := t.pod.namespacePath(specs.NetworkNamespace)
		if podNsPath != "" {
			t.g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, podNsPath)
		}
	}
	switch security.GetNamespaceOptions().GetPid() {
	case k8s.NamespaceMode_CONTAINER:
		t.g.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), "")
	case k8s.NamespaceMode_POD:
		podNsPath := t.pod.namespacePath(specs.PIDNamespace)
		if podNsPath != "" {
			t.g.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), podNsPath)
		}
	}
}

func (t *containerTranslator) configureResources() {
	res := t.cont.GetLinux().GetResources()
	t.g.SetLinuxResourcesCPUMems(res.GetCpusetMems())
	t.g.SetLinuxResourcesCPUCpus(res.GetCpusetCpus())

	if res.GetCpuPeriod() != 0 {
		t.g.SetLinuxResourcesCPUPeriod(uint64(res.GetCpuPeriod()))
	}
	if res.GetCpuQuota() != 0 {
		t.g.SetLinuxResourcesCPUQuota(res.GetCpuQuota())
	}
	if res.GetCpuShares() != 0 {
		t.g.SetLinuxResourcesCPUShares(uint64(res.GetCpuShares()))
	}
	if res.GetOomScoreAdj() != 0 {
		t.g.SetProcessOOMScoreAdj(int(res.GetOomScoreAdj()))
	}
	if res.GetMemoryLimitInBytes() != 0 {
		t.g.SetLinuxResourcesMemoryLimit(res.GetMemoryLimitInBytes())
	}
}

func (t *containerTranslator) configureProcess() error {
	const (
		runScript  = "/.singularity.d/runscript"
		execScript = "/.singularity.d/actions/exec"

		appArmorLocalhostPrefix = "localhost/"
	)

	for _, env := range t.cont.GetEnvs() {
		t.g.AddProcessEnv(env.GetKey(), env.GetValue())
	}
	t.g.SetProcessCwd(t.cont.GetWorkingDir())
	t.g.SetProcessTerminal(t.cont.GetTty())

	args := append(t.cont.GetCommand(), t.cont.GetArgs()...)
	if len(args) == 0 {
		args = []string{runScript}
	}
	t.g.SetProcessArgs(append([]string{execScript}, args...))

	security := t.cont.GetLinux().GetSecurityContext()
	t.g.SetProcessNoNewPrivileges(security.GetNoNewPrivs())
	t.g.SetProcessUsername(security.GetRunAsUsername())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}
	if security.GetPrivileged() {
		t.g.SetupPrivileged(true)
	} else {
		aaProfile := security.GetApparmorProfile()
		aaProfile = strings.TrimPrefix(aaProfile, appArmorLocalhostPrefix)
		t.g.SetProcessApparmorProfile(aaProfile)

		if err := setupSELinux(&t.g, security.GetSelinuxOptions()); err != nil {
			return err
		}

		for _, capb := range security.GetCapabilities().GetDropCapabilities() {
			t.g.DropProcessCapabilityEffective(capb)
			t.g.DropProcessCapabilityAmbient(capb)
			t.g.DropProcessCapabilityBounding(capb)
			t.g.DropProcessCapabilityInheritable(capb)
			t.g.DropProcessCapabilityPermitted(capb)
		}
		for _, capb := range security.GetCapabilities().GetAddCapabilities() {
			t.g.AddProcessCapabilityEffective(capb)
			t.g.AddProcessCapabilityAmbient(capb)
			t.g.AddProcessCapabilityBounding(capb)
			t.g.AddProcessCapabilityInheritable(capb)
			t.g.AddProcessCapabilityPermitted(capb)
		}
	}
	return nil
}

func (t *containerTranslator) configureAnnotations() {
	for k, v := range t.cont.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
}
