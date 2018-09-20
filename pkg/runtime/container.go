package runtime

import (
	"context"
	"fmt"

	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// CreateContainer creates a new container in specified PodSandbox
func (s *SingularityRuntime) CreateContainer(context.Context, *v1alpha2.CreateContainerRequest) (*v1alpha2.CreateContainerResponse, error) {
	return &v1alpha2.CreateContainerResponse{}, fmt.Errorf("not implemented")
}

// StartContainer starts the container.
func (s *SingularityRuntime) StartContainer(context.Context, *v1alpha2.StartContainerRequest) (*v1alpha2.StartContainerResponse, error) {
	return &v1alpha2.StartContainerResponse{}, fmt.Errorf("not implemented")
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
// TODO: what must the runtime do after the grace period is reached?
func (s *SingularityRuntime) StopContainer(context.Context, *v1alpha2.StopContainerRequest) (*v1alpha2.StopContainerResponse, error) {
	return &v1alpha2.StopContainerResponse{}, fmt.Errorf("not implemented")
}

// RemoveContainer removes the container. If the container is running, the
// container must be forcibly removed.
// This call is idempotent, and must not return an error if the container has
// already been removed.
func (s *SingularityRuntime) RemoveContainer(context.Context, *v1alpha2.RemoveContainerRequest) (*v1alpha2.RemoveContainerResponse, error) {
	return &v1alpha2.RemoveContainerResponse{}, fmt.Errorf("not implemented")
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(context.Context, *v1alpha2.ListContainersRequest) (*v1alpha2.ListContainersResponse, error) {
	return &v1alpha2.ListContainersResponse{}, fmt.Errorf("not implemented")
}

// ContainerStatus returns status of the container. If the container is not
// present, returns an error.
func (s *SingularityRuntime) ContainerStatus(context.Context, *v1alpha2.ContainerStatusRequest) (*v1alpha2.ContainerStatusResponse, error) {
	return &v1alpha2.ContainerStatusResponse{}, fmt.Errorf("not implemented")
}
