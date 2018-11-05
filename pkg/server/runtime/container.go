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
	"log"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/kube/container"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *k8s.CreateContainerRequest) (*k8s.CreateContainerResponse, error) {
	info, err := s.imageIndex.Find(req.Config.Image.Image)
	if err == image.ErrNotFound {
		return nil, status.Error(codes.NotFound, "image not found")
	}

	pod, err := s.findPod(req.PodSandboxId)
	if err != nil {
		return nil, err
	}

	cont := container.New(req.Config, pod)
	// add to trunc index first not to cleanup if it fails later
	err = s.containers.Add(cont)
	if err != nil {
		return nil, err
	}

	if err := cont.Create(info); err != nil {
		if err := s.containers.Remove(cont.ID()); err != nil {
			log.Printf("could not remove container from index: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "could not create container: %v", err)
	}
	return &k8s.CreateContainerResponse{
		ContainerId: cont.ID(),
	}, nil
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
	cont, err := s.containers.Find(req.ContainerId)
	if err == container.ErrNotFound {
		return nil, status.Errorf(codes.NotFound, "pod not found")
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	info, err := s.imageIndex.Find(cont.GetImage().GetImage())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not get container image: %v", cont.ID(), err)
	}
	return &k8s.ContainerStatusResponse{
		Status: &k8s.ContainerStatus{
			Id:          cont.ID(),
			Metadata:    cont.GetMetadata(),
			State:       0,
			CreatedAt:   cont.CreatedAt(),
			StartedAt:   0,
			FinishedAt:  0,
			ExitCode:    0,
			Image:       cont.GetImage(),
			ImageRef:    info.ID(),
			Labels:      cont.GetLabels(),
			Annotations: cont.GetAnnotations(),
			Mounts:      cont.GetMounts(),
			LogPath:     cont.GetLogPath(), // todo concat with pod log dir?
		},
	}, nil
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *k8s.ListContainersRequest) (*k8s.ListContainersResponse, error) {
	var containers []*k8s.Container

	appendContToResult := func(cont *container.Container) {
		if cont.MatchesFilter(req.Filter) {
			info, err := s.imageIndex.Find(cont.GetImage().GetImage())
			if err != nil {
				log.Printf("skipping container %s due to %v", cont.ID(), err)
				return
			}
			containers = append(containers, &k8s.Container{
				Id:           cont.ID(),
				PodSandboxId: cont.PodID(),
				Metadata:     cont.GetMetadata(),
				Image:        cont.GetImage(),
				ImageRef:     info.ID(),
				State:        cont.State(),
				CreatedAt:    cont.CreatedAt(),
				Labels:       cont.GetLabels(),
				Annotations:  cont.GetAnnotations(),
			})
		}
	}
	s.containers.Iterate(appendContToResult)
	return &k8s.ListContainersResponse{
		Containers: containers,
	}, nil

}
