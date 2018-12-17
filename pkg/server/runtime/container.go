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

	"github.com/sylabs/cri/pkg/index"
	"github.com/sylabs/cri/pkg/kube"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *k8s.CreateContainerRequest) (*k8s.CreateContainerResponse, error) {
	if req.GetConfig().GetTty() && !req.GetConfig().GetStdin() {
		return nil, status.Error(codes.InvalidArgument, "tty requires stdin to be true")
	}
	if req.GetConfig().GetLinux().GetSecurityContext().GetRunAsUser() != nil &&
		req.GetConfig().GetLinux().GetSecurityContext().GetRunAsUsername() != "" {
		return nil, status.Error(codes.InvalidArgument, "only one of RunAsUser and RunAsUsername can be specified at a time")
	}
	if req.GetConfig().GetLinux().GetSecurityContext().GetRunAsGroup() != nil &&
		req.GetConfig().GetLinux().GetSecurityContext().GetRunAsUser() == nil &&
		req.GetConfig().GetLinux().GetSecurityContext().GetRunAsUsername() == "" {
		return nil, status.Error(codes.InvalidArgument, "RunAsGroup should only be specified when RunAsUser or RunAsUsername is specified")
	}

	info, err := s.imageIndex.Find(req.Config.GetImage().GetImage())
	if err == index.ErrImageNotFound {
		return nil, status.Error(codes.NotFound, "image is not found")
	}

	pod, err := s.findPod(req.PodSandboxId)
	if err != nil {
		return nil, err
	}

	cont := kube.NewContainer(req.Config, pod)
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
	cont, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}

	err = cont.Start()
	if err == kube.ErrContainerNotCreated {
		return nil, status.Errorf(codes.InvalidArgument, "attempt to start container in %s state", cont.State())
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not start container: %v", err)
	}
	return &k8s.StartContainerResponse{}, nil
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped. If a grace period is reached runtime will be asked
// to kill container.
func (s *SingularityRuntime) StopContainer(_ context.Context, req *k8s.StopContainerRequest) (*k8s.StopContainerResponse, error) {
	cont, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}

	if err := cont.Stop(req.Timeout); err != nil {
		return nil, status.Errorf(codes.Internal, "could not stop container: %v", err)
	}
	return &k8s.StopContainerResponse{}, nil
}

// RemoveContainer removes the container. If the container is running,
// the container must be forcibly removed. This call is idempotent, and
// must not return an error if the container has already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *k8s.RemoveContainerRequest) (*k8s.RemoveContainerResponse, error) {
	cont, err := s.containers.Find(req.ContainerId)
	if err == index.ErrContainerNotFound {
		return &k8s.RemoveContainerResponse{}, nil
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := cont.Remove(); err != nil {
		return nil, status.Errorf(codes.Internal, "could not remove container: %v", err)
	}
	if err := s.containers.Remove(cont.ID()); err != nil {
		return nil, status.Errorf(codes.Internal, "could not remove container from index: %v", err)
	}
	return &k8s.RemoveContainerResponse{}, nil
}

// ContainerStatus returns status of the container.
// If the container is not present, returns an error.
func (s *SingularityRuntime) ContainerStatus(_ context.Context, req *k8s.ContainerStatusRequest) (*k8s.ContainerStatusResponse, error) {
	cont, err := s.findContainer(req.ContainerId)
	if err != nil {
		return nil, err
	}

	info, err := s.imageIndex.Find(cont.GetImage().GetImage())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not get container image: %v", err)
	}
	if err := cont.UpdateState(); err != nil {
		return nil, status.Errorf(codes.Internal, "could not update container state: %v", err)
	}

	var verboseInfo map[string]string
	if req.Verbose {
		verboseInfo = map[string]string{
			"pid": fmt.Sprintf("%d", cont.Pid()),
		}
	}
	return &k8s.ContainerStatusResponse{
		Status: &k8s.ContainerStatus{
			Id:          cont.ID(),
			Metadata:    cont.GetMetadata(),
			State:       cont.State(),
			CreatedAt:   cont.CreatedAt(),
			StartedAt:   cont.StartedAt(),
			FinishedAt:  cont.FinishedAt(),
			ExitCode:    cont.ExitCode(),
			Image:       cont.GetImage(),
			ImageRef:    info.ID(),
			Reason:      cont.ExitDescription(),
			Message:     cont.ExitDescription(),
			Labels:      cont.GetLabels(),
			Annotations: cont.GetAnnotations(),
			Mounts:      cont.GetMounts(),
			LogPath:     cont.LogPath(),
		},
		Info: verboseInfo,
	}, nil
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *k8s.ListContainersRequest) (*k8s.ListContainersResponse, error) {
	var containers []*k8s.Container

	appendContToResult := func(cont *kube.Container) {
		if err := cont.UpdateState(); err != nil {
			log.Printf("could not fetch container %s: %v", cont.ID(), err)
			return
		}
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

func (s *SingularityRuntime) findContainer(id string) (*kube.Container, error) {
	cont, err := s.containers.Find(id)
	if err == index.ErrContainerNotFound {
		return nil, status.Error(codes.NotFound, "container is not found")
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return cont, nil
}
