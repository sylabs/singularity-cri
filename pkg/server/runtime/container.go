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
	"encoding/json"
	"log"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/oci/translate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *k8s.CreateContainerRequest) (*k8s.CreateContainerResponse, error) {
	pod, err := s.findPod(req.PodSandboxId)
	if err != nil {
		return nil, err
	}

	// todo oci bundle here
	info, err := s.imageIndex.Find(req.Config.Image.Image)
	if err == image.ErrNotFound {
		return nil, status.Error(codes.NotFound, "image not found")
	}

	req.Config.Image.Image = info.Path() // a hack for starter to work correctly
	ociConfig, err := translate.KubeToOCI(req.Config, pod)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not generate OCI config: %v", err)
	}

	ociBytes, err := json.Marshal(ociConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not marshal OCI config: %v", err)
	}
	log.Printf("OCI config is: %s", ociBytes)

	// todo create log file
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// StartContainer starts the container.
func (s *SingularityRuntime) StartContainer(_ context.Context, req *k8s.StartContainerRequest) (*k8s.StartContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
// TODO: what must the runtime do after the grace period is reached?
func (s *SingularityRuntime) StopContainer(_ context.Context, req *k8s.StopContainerRequest) (*k8s.StopContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// RemoveContainer removes the container. If the container is running,
// the container must be forcibly removed. This call is idempotent, and
// must not return an error if the container has already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *k8s.RemoveContainerRequest) (*k8s.RemoveContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// ContainerStatus returns status of the container.
// If the container is not present, returns an error.
func (s *SingularityRuntime) ContainerStatus(_ context.Context, req *k8s.ContainerStatusRequest) (*k8s.ContainerStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *k8s.ListContainersRequest) (*k8s.ListContainersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}
