package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sylabs/singularity/src/pkg/instance"
	"github.com/sylabs/singularity/src/pkg/sylog"
	syexec "github.com/sylabs/singularity/src/pkg/util/exec"
	"github.com/sylabs/singularity/src/runtime/engines/config"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type container struct {
	id        string
	podID     string
	config    *v1alpha2.ContainerConfig
	createdAt int64
	startedAt int64
	state     v1alpha2.ContainerState
}

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *v1alpha2.CreateContainerRequest) (*v1alpha2.CreateContainerResponse, error) {
	meta := req.Config.Metadata // assume metadata is always non-nil
	containerID := fmt.Sprintf("%s_%s_%d", req.PodSandboxId, meta.Name, meta.Attempt)
	req.Config.Image.Image = s.registry.ImagePath(req.Config.Image.Image)

	engineConf := config.Common{
		EngineName:   "kube_container",
		EngineConfig: req,
	}
	configData, err := json.Marshal(engineConf)
	if err != nil {
		return nil, fmt.Errorf("could not marshal engine config: %s", err)
	}

	envs := []string{sylog.GetEnvVar(), fmt.Sprintf("SRUNTIME=%s", engineConf.EngineName)}
	pipefd, err := syexec.SetPipe(configData)
	if err != nil {
		return nil, fmt.Errorf("could not configure pipe: %v", err)
	}
	envs = append(envs, pipefd)

	cmd := exec.Command("starter", containerID)
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Println("will start container now")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not start container: %s", err)
	}
	log.Println("container started")

	cont := container{
		id:        containerID,
		podID:     req.PodSandboxId,
		config:    req.Config,
		createdAt: time.Now().Unix(),
		state:     v1alpha2.ContainerState_CONTAINER_CREATED,
	}

	s.pMu.RLock()
	pod := s.pods[req.PodSandboxId]
	s.pMu.RUnlock()
	pod.containers = append(pod.containers, containerID)
	s.pMu.Lock()
	s.pods[req.PodSandboxId] = pod
	s.pMu.Unlock()

	s.cMu.Lock()
	s.containers[containerID] = cont
	s.cMu.Unlock()
	return &v1alpha2.CreateContainerResponse{
		ContainerId: containerID,
	}, nil
}

// StartContainer starts the container.
func (s *SingularityRuntime) StartContainer(_ context.Context, req *v1alpha2.StartContainerRequest) (*v1alpha2.StartContainerResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	contI, err := instance.Get(cont.id)
	if err != nil {
		return nil, fmt.Errorf("could not read container instance file: %v", err)
	}

	err = syscall.Kill(contI.Pid, syscall.SIGCONT)
	if err != nil {
		return nil, fmt.Errorf("could not start container: %v", err)
	}

	cont.state = v1alpha2.ContainerState_CONTAINER_RUNNING
	cont.startedAt = time.Now().Unix()
	s.cMu.Lock()
	s.containers[cont.id] = cont
	s.cMu.Unlock()
	return &v1alpha2.StartContainerResponse{}, nil
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
// TODO: what must the runtime do after the grace period is reached?
func (s *SingularityRuntime) StopContainer(_ context.Context, req *v1alpha2.StopContainerRequest) (*v1alpha2.StopContainerResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if cont.state == v1alpha2.ContainerState_CONTAINER_CREATED {
		return &v1alpha2.StopContainerResponse{}, nil
	}

	contI, err := instance.Get(cont.id)
	if err != nil {
		return nil, fmt.Errorf("could not read container instance file: %v", err)
	}

	err = syscall.Kill(contI.Pid, syscall.SIGSTOP)
	if err != nil {
		return nil, fmt.Errorf("could not stop container: %v", err)
	}

	cont.state = v1alpha2.ContainerState_CONTAINER_CREATED
	s.cMu.Lock()
	s.containers[cont.id] = cont
	s.cMu.Unlock()

	return &v1alpha2.StopContainerResponse{}, nil
}

// RemoveContainer removes the container. If the container is running, the
// container must be forcibly removed.
// This call is idempotent, and must not return an error if the container has
// already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *v1alpha2.RemoveContainerRequest) (*v1alpha2.RemoveContainerResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return &v1alpha2.RemoveContainerResponse{}, nil
	}
	contI, err := instance.Get(cont.id)
	if err != nil {
		return nil, fmt.Errorf("could not read container instance file: %v", err)
	}

	err = syscall.Kill(contI.Pid, syscall.SIGKILL)
	if err != nil {
		return nil, fmt.Errorf("could not kill container: %v", err)
	}

	for err != syscall.ESRCH {
		// todo think how this may be optimized
		err = syscall.Kill(contI.PPid, syscall.SIGKILL)
	}
	log.Println("monitor exited")

	_, err = os.Stat(contI.Path)
	if !os.IsNotExist(err) {
		err := os.Remove(contI.Path)
		if err != nil {
			return nil, fmt.Errorf("could not remove container instance file: %v", err)
		}
	}

	s.cMu.Lock()
	delete(s.containers, cont.id)
	s.cMu.Unlock()
	return &v1alpha2.RemoveContainerResponse{}, nil
}

// ContainerStatus returns status of the container. If the container is not
// present, returns an error.
func (s *SingularityRuntime) ContainerStatus(_ context.Context, req *v1alpha2.ContainerStatusRequest) (*v1alpha2.ContainerStatusResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	return &v1alpha2.ContainerStatusResponse{
		Status: &v1alpha2.ContainerStatus{
			Id:          req.ContainerId,
			Metadata:    cont.config.Metadata,
			State:       cont.state,
			CreatedAt:   cont.createdAt,
			StartedAt:   cont.startedAt,
			FinishedAt:  0,
			ExitCode:    0,
			Image:       cont.config.Image,
			ImageRef:    cont.config.Image.Image,
			Reason:      "",
			Message:     "",
			Labels:      cont.config.Labels,
			Annotations: cont.config.Annotations,
			Mounts:      cont.config.Mounts,
			LogPath:     "",
		},
	}, nil
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *v1alpha2.ListContainersRequest) (*v1alpha2.ListContainersResponse, error) {
	resp := &v1alpha2.ListContainersResponse{}
	s.cMu.RLock()
	defer s.cMu.RUnlock()
	for _, cont := range s.containers {
		if containerMatches(cont, req.Filter) {
			resp.Containers = append(resp.Containers, &v1alpha2.Container{
				Id:           cont.id,
				PodSandboxId: cont.podID,
				Metadata:     cont.config.Metadata,
				Image:        cont.config.Image,
				ImageRef:     cont.config.Image.Image,
				State:        cont.state,
				CreatedAt:    cont.createdAt,
				Labels:       cont.config.Labels,
				Annotations:  cont.config.Annotations,
			})
		}
	}
	return resp, nil
}

func containerMatches(cont container, filter *v1alpha2.ContainerFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != cont.id {
		return false
	}
	if filter.PodSandboxId != "" && filter.PodSandboxId != cont.podID {
		return false
	}
	if filter.State != nil && filter.State.State != cont.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := cont.config.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
