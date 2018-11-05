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

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/kube/container"
	"github.com/sylabs/cri/pkg/kube/sandbox"
	"github.com/sylabs/cri/pkg/singularity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SingularityRuntime implements k8s RuntimeService interface.
type SingularityRuntime struct {
	singularity string
	starter     string
	imageIndex  *image.Index
	pods        *sandbox.Index
	containers  *container.Index
}

// NewSingularityRuntime initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
// SingularityRuntime depends on SingularityRegistry so it must not be nil.
func NewSingularityRuntime(index *image.Index) (*SingularityRuntime, error) {
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
		imageIndex:  index,
		pods:        sandbox.NewIndex(),
		containers:  container.NewIndex(),
	}, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (s *SingularityRuntime) Version(context.Context, *k8s.VersionRequest) (*k8s.VersionResponse, error) {
	const kubeAPIVersion = "0.1.0"

	syVersion, err := exec.Command(s.singularity, "version").Output()
	if err != nil {
		return nil, err
	}

	return &k8s.VersionResponse{
		Version:           kubeAPIVersion, // todo or use req.Version?
		RuntimeName:       singularity.RuntimeName,
		RuntimeVersion:    string(syVersion),
		RuntimeApiVersion: string(syVersion),
	}, nil
}

// UpdateContainerResources updates ContainerConfig of the container.
func (s *SingularityRuntime) UpdateContainerResources(context.Context, *k8s.UpdateContainerResourcesRequest) (*k8s.UpdateContainerResourcesResponse, error) {
	return &k8s.UpdateContainerResourcesResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// ReopenContainerLog asks runtime to reopen the stdout/stderr log file
// for the container. This is often called after the log file has been
// rotated. If the container is not running, container runtime can choose
// to either create a new log file and return nil, or return an error.
// Once it returns error, new container log file MUST NOT be created.
func (s *SingularityRuntime) ReopenContainerLog(context.Context, *k8s.ReopenContainerLogRequest) (*k8s.ReopenContainerLogResponse, error) {
	return &k8s.ReopenContainerLogResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// ExecSync runs a command in a container synchronously.
func (s *SingularityRuntime) ExecSync(context.Context, *k8s.ExecSyncRequest) (*k8s.ExecSyncResponse, error) {
	return &k8s.ExecSyncResponse{}, status.Errorf(codes.Unimplemented, "EXEC SYNC not implemented")
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (s *SingularityRuntime) Exec(context.Context, *k8s.ExecRequest) (*k8s.ExecResponse, error) {
	return &k8s.ExecResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (s *SingularityRuntime) Attach(context.Context, *k8s.AttachRequest) (*k8s.AttachResponse, error) {
	return &k8s.AttachResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *SingularityRuntime) PortForward(context.Context, *k8s.PortForwardRequest) (*k8s.PortForwardResponse, error) {
	return &k8s.PortForwardResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (s *SingularityRuntime) ContainerStats(context.Context, *k8s.ContainerStatsRequest) (*k8s.ContainerStatsResponse, error) {
	return &k8s.ContainerStatsResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// ListContainerStats returns stats of all running containers.
func (s *SingularityRuntime) ListContainerStats(context.Context, *k8s.ListContainerStatsRequest) (*k8s.ListContainerStatsResponse, error) {
	return &k8s.ListContainerStatsResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// UpdateRuntimeConfig updates the runtime configuration based on the given request.
func (s *SingularityRuntime) UpdateRuntimeConfig(context.Context, *k8s.UpdateRuntimeConfigRequest) (*k8s.UpdateRuntimeConfigResponse, error) {
	return &k8s.UpdateRuntimeConfigResponse{}, status.Errorf(codes.Unimplemented, "not implemented")
}

// Status returns the status of the runtime.
func (s *SingularityRuntime) Status(ctx context.Context, req *k8s.StatusRequest) (*k8s.StatusResponse, error) {
	runtimeReady := &k8s.RuntimeCondition{
		Type:   k8s.RuntimeReady,
		Status: true,
	}
	networkReady := &k8s.RuntimeCondition{
		Type:   k8s.NetworkReady,
		Status: true,
	}
	conditions := []*k8s.RuntimeCondition{runtimeReady, networkReady}

	return &k8s.StatusResponse{
		Status: &k8s.RuntimeStatus{
			Conditions: conditions,
		},
	}, nil
}
