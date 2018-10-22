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

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type container struct {
	id      string
	config  *k8s.ContainerConfig
	logPath string
}

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *k8s.CreateContainerRequest) (*k8s.CreateContainerResponse, error) {
	pod := s.findPod(req.PodSandboxId)
	if pod == nil {
		return nil, status.Error(codes.NotFound, "pod not found")
	}
	cont := &container{
		id:     containerID(pod.id, req.GetConfig().GetMetadata()),
		config: req.GetConfig(),
	}

	gen, err := generate.New("linux")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not create generator: %v", err)
	}
	if pod.config.GetHostname() != "" {
		gen.SetHostname(pod.config.GetHostname())
		gen.AddMount(specs.Mount{
			Destination: "/etc/hostname",
			Source:      hostnameFilePath(pod.id),
			Options:     []string{"bind", "ro"},
		})
		gen.AddOrReplaceLinuxNamespace(specs.UTSNamespace, bindNamespacePath(pod.id, specs.UTSNamespace))
	}
	if pod.config.GetDnsConfig() != nil {
		gen.AddMount(specs.Mount{
			Destination: "/etc/resolv.conf",
			Source:      resolvConfFilePath(pod.id),
			Options:     []string{"bind", "ro"},
		})
	}

	gen.AddOrReplaceLinuxNamespace(specs.MountNamespace, "")
	gen.AddOrReplaceLinuxNamespace(string(specs.PIDNamespace), "")

	security := cont.config.GetLinux().GetSecurityContext()
	gen.SetProcessApparmorProfile(security.GetApparmorProfile())
	gen.SetProcessNoNewPrivileges(security.NoNewPrivs)
	gen.SetRootReadonly(security.GetReadonlyRootfs())
	gen.SetProcessUsername(security.GetRunAsUsername())
	gen.SetProcessUID(uint32(security.GetRunAsUser().Value))
	gen.SetProcessGID(uint32(security.GetRunAsGroup().Value))
	for _, gid := range security.GetSupplementalGroups() {
		gen.AddProcessAdditionalGid(uint32(gid))
	}
	for _, capb := range security.GetCapabilities().DropCapabilities {
		gen.DropProcessCapabilityEffective(capb)
	}
	for _, capb := range security.GetCapabilities().AddCapabilities {
		gen.AddProcessCapabilityEffective(capb)
	}
	for k, v := range pod.config.GetLinux().GetSysctls() {
		gen.AddLinuxSysctl(k, v)
	}

	switch security.GetNamespaceOptions().GetIpc() {
	case k8s.NamespaceMode_CONTAINER:
		gen.AddOrReplaceLinuxNamespace(specs.IPCNamespace, "")
	case k8s.NamespaceMode_POD:
		gen.AddOrReplaceLinuxNamespace(specs.IPCNamespace, bindNamespacePath(pod.id, specs.IPCNamespace))

	}
	switch security.GetNamespaceOptions().GetNetwork() {
	case k8s.NamespaceMode_CONTAINER:
		gen.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, "")
	case k8s.NamespaceMode_POD:
		gen.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, bindNamespacePath(pod.id, specs.NetworkNamespace))
	}

	if cont.config.GetLogPath() != "" {
		logPath := filepath.Join(pod.config.LogDirectory, cont.config.GetLogPath())
		err := os.MkdirAll(filepath.Dir(logPath), os.ModePerm)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not create log directory: %v", err)
		}
		logs, err := os.Create(logPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not create log file: %v", err)
		}
		logs.Close()
		cont.logPath = logPath
	}

	for k, v := range cont.config.GetAnnotations() {
		gen.AddAnnotation(k, v)
	}
	for _, env := range cont.config.GetEnvs() {
		gen.AddProcessEnv(env.Key, env.Value)
	}
	for _, mount := range cont.config.GetMounts() {
		volume := specs.Mount{
			Destination: mount.GetContainerPath(),
			Type:        "",
			Source:      mount.GetHostPath(),
			Options:     []string{"bind"},
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
		gen.AddMount(volume)
	}

	res := cont.config.GetLinux().GetResources()
	gen.SetLinuxResourcesCPUPeriod(uint64(res.GetCpuPeriod()))
	gen.SetLinuxResourcesCPUQuota(res.GetCpuQuota())
	gen.SetLinuxResourcesCPUMems(res.GetCpusetMems())
	gen.SetLinuxResourcesCPUCpus(res.GetCpusetCpus())
	gen.SetLinuxResourcesCPUShares(uint64(res.GetCpuShares()))
	gen.SetProcessOOMScoreAdj(int(res.GetOomScoreAdj()))
	gen.SetLinuxResourcesMemoryLimit(res.GetMemoryLimitInBytes())

	gen.SetProcessCwd(cont.config.GetWorkingDir())
	gen.SetProcessArgs(append(cont.config.GetCommand(), cont.config.GetArgs()...))
	gen.SetProcessTerminal(cont.config.GetTty())

	return nil, nil
}

// StartContainer starts the container.
func (s *SingularityRuntime) StartContainer(_ context.Context, req *k8s.StartContainerRequest) (*k8s.StartContainerResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
// TODO: what must the runtime do after the grace period is reached?
func (s *SingularityRuntime) StopContainer(_ context.Context, req *k8s.StopContainerRequest) (*k8s.StopContainerResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// RemoveContainer removes the container. If the container is running,
// the container must be forcibly removed. This call is idempotent, and
// must not return an error if the container has already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *k8s.RemoveContainerRequest) (*k8s.RemoveContainerResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ContainerStatus returns status of the container.
// If the container is not present, returns an error.
func (s *SingularityRuntime) ContainerStatus(_ context.Context, req *k8s.ContainerStatusRequest) (*k8s.ContainerStatusResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *k8s.ListContainersRequest) (*k8s.ListContainersResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
