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
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/sylabs/cri/pkg/index"
	"github.com/sylabs/cri/pkg/singularity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

// SingularityRuntime implements k8s RuntimeService interface.
type SingularityRuntime struct {
	singularity string
	starter     string
	imageIndex  *index.ImageIndex
	pods        *index.PodIndex
	containers  *index.ContainerIndex

	streaming streaming.Server
}

// NewSingularityRuntime initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
// SingularityRuntime depends on SingularityRegistry so it must not be nil.
func NewSingularityRuntime(streamURL string, imgIndex *index.ImageIndex) (*SingularityRuntime, error) {
	sing, err := exec.LookPath(singularity.RuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s on this machine: %v", singularity.RuntimeName, err)
	}
	start, err := exec.LookPath(singularity.StarterName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s on this machine: %v", singularity.StarterName, err)
	}

	runtime := &SingularityRuntime{
		singularity: sing,
		starter:     start,
		imageIndex:  imgIndex,
		pods:        index.NewPodIndex(),
		containers:  index.NewContainerIndex(),
	}
	streamingRuntime := &streamingRuntime{runtime}

	streamingConfig := streaming.DefaultConfig
	streamingConfig.Addr = streamURL
	streamingServer, err := streaming.NewServer(streamingConfig, streamingRuntime)
	if err != nil {
		return nil, fmt.Errorf("could not create streaming server: %v", err)
	}

	go func() {
		err := streamingServer.Start(true)
		if err != nil && err != http.ErrServerClosed {
			log.Printf("streaming server error: %v", err)
		}
	}()

	runtime.streaming = streamingServer
	return runtime, nil
}

// Shutdown shuts down any running background tasks created by SingularityRuntime.
// This methods should be called when SingularityRuntime will no longer be used.
func (s *SingularityRuntime) Shutdown() error {
	if err := s.streaming.Stop(); err != nil {
		return fmt.Errorf("could not stop streaming server: %v", err)
	}
	return nil
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
func (s *SingularityRuntime) ReopenContainerLog(ctx context.Context, req *k8s.ReopenContainerLogRequest) (*k8s.ReopenContainerLogResponse, error) {
	cont, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}
	if cont.State() != k8s.ContainerState_CONTAINER_RUNNING {
		return nil, status.Error(codes.InvalidArgument, "container is not running")
	}

	err = cont.ReopenLogFile()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not reopen log file: %v", err)
	}
	return &k8s.ReopenContainerLogResponse{}, nil
}

// ExecSync runs a command in a container synchronously.
func (s *SingularityRuntime) ExecSync(ctx context.Context, req *k8s.ExecSyncRequest) (*k8s.ExecSyncResponse, error) {
	cont, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}

	timeout := time.Second * time.Duration(req.Timeout)
	resp, err := cont.ExecSync(timeout, req.Cmd)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not execute in container: %v", err)
	}
	return resp, nil
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (s *SingularityRuntime) Exec(ctx context.Context, req *k8s.ExecRequest) (*k8s.ExecResponse, error) {
	_, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}
	if !(req.GetStdout() || req.GetStderr() || req.GetStdin()) {
		return nil, status.Error(codes.InvalidArgument, "One of `stdin`, `stdout`, and `stderr` MUST be true")
	}
	if req.GetTty() && req.GetStderr() {
		return nil, status.Error(codes.InvalidArgument, "If `tty` is true, `stderr` MUST be false")
	}
	return s.streaming.GetExec(req)
}

// Attach prepares a streaming endpoint to attach to a running container.
func (s *SingularityRuntime) Attach(ctx context.Context, req *k8s.AttachRequest) (*k8s.AttachResponse, error) {
	c, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}
	if c.GetTty() != req.GetTty() {
		return nil, status.Error(codes.InvalidArgument, "tty doesn't match container configuration")
	}
	if !(req.GetStdout() || req.GetStderr() || req.GetStdin()) {
		return nil, status.Error(codes.InvalidArgument, "One of `stdin`, `stdout`, and `stderr` MUST be true")
	}
	if req.GetTty() && req.GetStderr() {
		return nil, status.Error(codes.InvalidArgument, "If `tty` is true, `stderr` MUST be false")
	}
	return s.streaming.GetAttach(req)
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *SingularityRuntime) PortForward(ctx context.Context, req *k8s.PortForwardRequest) (*k8s.PortForwardResponse, error) {
	_, err := s.findPod(req.PodSandboxId)
	if err != nil {
		return nil, err
	}
	return s.streaming.GetPortForward(req)
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
