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
	"os/exec"
	"sync"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/singularity"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SingularityRuntime implements k8s RuntimeService interface.
type SingularityRuntime struct {
	singularity string
	starter     string
	registry    *image.SingularityRegistry

	pMu  sync.RWMutex
	pods map[string]pod

	cMu        sync.RWMutex
	containers map[string]container
}

// NewSingularityRuntime initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
// SingularityRuntime depends on SingularityRegistry so it must not be nil.
func NewSingularityRuntime(registry *image.SingularityRegistry) (*SingularityRuntime, error) {
	sing, err := exec.LookPath(singularity.RuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s on this machine: %v", singularity.RuntimeName, err)
	}
	start, err := exec.LookPath(singularity.StarterName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s on this machine: %v", singularity.StarterName, err)
	}
	return &SingularityRuntime{
		singularity: sing,
		starter:     start,
		registry:    registry,
		pods:        make(map[string]pod),
		containers:  make(map[string]container),
	}, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (s *SingularityRuntime) Version(_ context.Context, _ *v1alpha2.VersionRequest) (*v1alpha2.VersionResponse, error) {
	const kubeAPIVersion = "0.1.0"

	syVersion, err := exec.Command(s.singularity, "version").Output()
	if err != nil {
		return nil, err
	}

	return &v1alpha2.VersionResponse{
		Version:           kubeAPIVersion, // todo or use req.Version?
		RuntimeName:       singularity.RuntimeName,
		RuntimeVersion:    string(syVersion),
		RuntimeApiVersion: string(syVersion),
	}, nil
}

// UpdateContainerResources updates ContainerConfig of the container.
func (s *SingularityRuntime) UpdateContainerResources(context.Context, *v1alpha2.UpdateContainerResourcesRequest) (*v1alpha2.UpdateContainerResourcesResponse, error) {
	return &v1alpha2.UpdateContainerResourcesResponse{}, fmt.Errorf("not implemented")
}

// ReopenContainerLog asks runtime to reopen the stdout/stderr log file
// for the container. This is often called after the log file has been
// rotated. If the container is not running, container runtime can choose
// to either create a new log file and return nil, or return an error.
// Once it returns error, new container log file MUST NOT be created.
func (s *SingularityRuntime) ReopenContainerLog(context.Context, *v1alpha2.ReopenContainerLogRequest) (*v1alpha2.ReopenContainerLogResponse, error) {
	return &v1alpha2.ReopenContainerLogResponse{}, fmt.Errorf("not implemented")
}

// ExecSync runs a command in a container synchronously.
func (s *SingularityRuntime) ExecSync(context.Context, *v1alpha2.ExecSyncRequest) (*v1alpha2.ExecSyncResponse, error) {
	return &v1alpha2.ExecSyncResponse{}, fmt.Errorf("EXEC SYNC not implemented")
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (s *SingularityRuntime) Exec(context.Context, *v1alpha2.ExecRequest) (*v1alpha2.ExecResponse, error) {
	return &v1alpha2.ExecResponse{}, fmt.Errorf("not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (s *SingularityRuntime) Attach(context.Context, *v1alpha2.AttachRequest) (*v1alpha2.AttachResponse, error) {
	return &v1alpha2.AttachResponse{}, fmt.Errorf("not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *SingularityRuntime) PortForward(context.Context, *v1alpha2.PortForwardRequest) (*v1alpha2.PortForwardResponse, error) {
	return &v1alpha2.PortForwardResponse{}, fmt.Errorf("not implemented")
}

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (s *SingularityRuntime) ContainerStats(context.Context, *v1alpha2.ContainerStatsRequest) (*v1alpha2.ContainerStatsResponse, error) {
	return &v1alpha2.ContainerStatsResponse{}, fmt.Errorf("not implemented")
}

// ListContainerStats returns stats of all running containers.
func (s *SingularityRuntime) ListContainerStats(context.Context, *v1alpha2.ListContainerStatsRequest) (*v1alpha2.ListContainerStatsResponse, error) {
	return &v1alpha2.ListContainerStatsResponse{}, fmt.Errorf("not implemented")
}

// UpdateRuntimeConfig updates the runtime configuration based on the given request.
func (s *SingularityRuntime) UpdateRuntimeConfig(context.Context, *v1alpha2.UpdateRuntimeConfigRequest) (*v1alpha2.UpdateRuntimeConfigResponse, error) {
	return &v1alpha2.UpdateRuntimeConfigResponse{}, fmt.Errorf("not implemented")
}

// Status returns the status of the runtime.
func (s *SingularityRuntime) Status(ctx context.Context, req *v1alpha2.StatusRequest) (*v1alpha2.StatusResponse, error) {
	runtimeReady := &v1alpha2.RuntimeCondition{
		Type:   v1alpha2.RuntimeReady,
		Status: true,
	}
	networkReady := &v1alpha2.RuntimeCondition{
		Type:   v1alpha2.NetworkReady,
		Status: true,
	}
	conditions := []*v1alpha2.RuntimeCondition{runtimeReady, networkReady}

	status := &v1alpha2.RuntimeStatus{Conditions: conditions}
	return &v1alpha2.StatusResponse{Status: status}, nil
}
