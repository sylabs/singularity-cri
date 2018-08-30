// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sycri

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	singularityRuntimeName = "singularity"
	kubeAPIVersion         = "0.1.0"
)

// SingularityRuntimeService implements k8s RuntimeService interface.
type SingularityRuntimeService struct {
	singularity string
}

// NewSingularityRuntimeService initializes and returns SingularityRuntimeService.
// Singularity must be installed on the host otherwise it will return an error.
func NewSingularityRuntimeService() (*SingularityRuntimeService, error) {
	singularity, err := exec.LookPath(singularityRuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s daemon on this machine: %v", singularityRuntimeName, err)
	}
	return &SingularityRuntimeService{
		singularity: singularity,
	}, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (s *SingularityRuntimeService) Version(context.Context, *v1alpha2.VersionRequest) (*v1alpha2.VersionResponse, error) {
	log.Println("SingularityRuntimeService.Version")
	syVersion, err := exec.Command(s.singularity, "version").Output()
	if err != nil {
		return nil, err
	}

	return &v1alpha2.VersionResponse{
		Version:           kubeAPIVersion, // todo or use req.Version?
		RuntimeName:       singularityRuntimeName,
		RuntimeVersion:    string(syVersion),
		RuntimeApiVersion: string(syVersion),
	}, nil
}

func (s *SingularityRuntimeService) RunPodSandbox(context.Context, *v1alpha2.RunPodSandboxRequest) (*v1alpha2.RunPodSandboxResponse, error) {
	log.Println("SingularityRuntimeService.RunPodSandbox")
	return &v1alpha2.RunPodSandboxResponse{}, nil
}

func (s *SingularityRuntimeService) StopPodSandbox(context.Context, *v1alpha2.StopPodSandboxRequest) (*v1alpha2.StopPodSandboxResponse, error) {
	log.Println("SingularityRuntimeService.StopPodSandbox")
	return &v1alpha2.StopPodSandboxResponse{}, nil
}

func (s *SingularityRuntimeService) RemovePodSandbox(context.Context, *v1alpha2.RemovePodSandboxRequest) (*v1alpha2.RemovePodSandboxResponse, error) {
	log.Println("SingularityRuntimeService.RemovePodSandbox")
	return &v1alpha2.RemovePodSandboxResponse{}, nil
}

func (s *SingularityRuntimeService) PodSandboxStatus(context.Context, *v1alpha2.PodSandboxStatusRequest) (*v1alpha2.PodSandboxStatusResponse, error) {
	log.Println("SingularityRuntimeService.PodSandboxStatus")
	return &v1alpha2.PodSandboxStatusResponse{}, nil
}

func (s *SingularityRuntimeService) ListPodSandbox(context.Context, *v1alpha2.ListPodSandboxRequest) (*v1alpha2.ListPodSandboxResponse, error) {
	log.Println("SingularityRuntimeService.ListPodSandbox")
	return &v1alpha2.ListPodSandboxResponse{}, nil
}

func (s *SingularityRuntimeService) CreateContainer(context.Context, *v1alpha2.CreateContainerRequest) (*v1alpha2.CreateContainerResponse, error) {
	log.Println("SingularityRuntimeService.CreateContainer")
	return &v1alpha2.CreateContainerResponse{}, nil
}

func (s *SingularityRuntimeService) StartContainer(context.Context, *v1alpha2.StartContainerRequest) (*v1alpha2.StartContainerResponse, error) {
	log.Println("SingularityRuntimeService.StartContainer")
	return &v1alpha2.StartContainerResponse{}, nil
}

func (s *SingularityRuntimeService) StopContainer(context.Context, *v1alpha2.StopContainerRequest) (*v1alpha2.StopContainerResponse, error) {
	log.Println("SingularityRuntimeService.StopContainer")
	return &v1alpha2.StopContainerResponse{}, nil
}

func (s *SingularityRuntimeService) RemoveContainer(context.Context, *v1alpha2.RemoveContainerRequest) (*v1alpha2.RemoveContainerResponse, error) {
	log.Println("SingularityRuntimeService.RemoveContainer")
	return &v1alpha2.RemoveContainerResponse{}, nil
}

func (s *SingularityRuntimeService) ListContainers(context.Context, *v1alpha2.ListContainersRequest) (*v1alpha2.ListContainersResponse, error) {
	log.Println("SingularityRuntimeService.ListContainers")
	return &v1alpha2.ListContainersResponse{}, nil
}

func (s *SingularityRuntimeService) ContainerStatus(context.Context, *v1alpha2.ContainerStatusRequest) (*v1alpha2.ContainerStatusResponse, error) {
	log.Println("SingularityRuntimeService.ContainerStatus")
	return &v1alpha2.ContainerStatusResponse{}, nil
}

func (s *SingularityRuntimeService) UpdateContainerResources(context.Context, *v1alpha2.UpdateContainerResourcesRequest) (*v1alpha2.UpdateContainerResourcesResponse, error) {
	log.Println("SingularityRuntimeService.UpdateContainerResources")
	return &v1alpha2.UpdateContainerResourcesResponse{}, nil
}

func (s *SingularityRuntimeService) ReopenContainerLog(context.Context, *v1alpha2.ReopenContainerLogRequest) (*v1alpha2.ReopenContainerLogResponse, error) {
	log.Println("SingularityRuntimeService.ReopenContainerLog")
	return &v1alpha2.ReopenContainerLogResponse{}, nil
}

func (s *SingularityRuntimeService) ExecSync(context.Context, *v1alpha2.ExecSyncRequest) (*v1alpha2.ExecSyncResponse, error) {
	log.Println("SingularityRuntimeService.ExecSync")
	return &v1alpha2.ExecSyncResponse{}, nil
}

func (s *SingularityRuntimeService) Exec(context.Context, *v1alpha2.ExecRequest) (*v1alpha2.ExecResponse, error) {
	log.Println("SingularityRuntimeService.Exec")
	return &v1alpha2.ExecResponse{}, nil
}

func (s *SingularityRuntimeService) Attach(context.Context, *v1alpha2.AttachRequest) (*v1alpha2.AttachResponse, error) {
	log.Println("SingularityRuntimeService.Attach")
	return &v1alpha2.AttachResponse{}, nil
}

func (s *SingularityRuntimeService) PortForward(context.Context, *v1alpha2.PortForwardRequest) (*v1alpha2.PortForwardResponse, error) {
	log.Println("SingularityRuntimeService.PortForward")
	return &v1alpha2.PortForwardResponse{}, nil
}

func (s *SingularityRuntimeService) ContainerStats(context.Context, *v1alpha2.ContainerStatsRequest) (*v1alpha2.ContainerStatsResponse, error) {
	log.Println("SingularityRuntimeService.ContainerStats")
	return &v1alpha2.ContainerStatsResponse{}, nil
}

func (s *SingularityRuntimeService) ListContainerStats(context.Context, *v1alpha2.ListContainerStatsRequest) (*v1alpha2.ListContainerStatsResponse, error) {
	log.Println("SingularityRuntimeService.ListContainerStats")
	return &v1alpha2.ListContainerStatsResponse{}, nil
}

func (s *SingularityRuntimeService) UpdateRuntimeConfig(context.Context, *v1alpha2.UpdateRuntimeConfigRequest) (*v1alpha2.UpdateRuntimeConfigResponse, error) {
	log.Println("SingularityRuntimeService.UpdateRuntimeConfig")
	return &v1alpha2.UpdateRuntimeConfigResponse{}, nil
}

func (s *SingularityRuntimeService) Status(context.Context, *v1alpha2.StatusRequest) (*v1alpha2.StatusResponse, error) {
	log.Println("SingularityRuntimeService.Status")
	return &v1alpha2.StatusResponse{}, nil
}
