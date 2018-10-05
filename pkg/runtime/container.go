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
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sylabs/singularity/src/pkg/sylog"
	syexec "github.com/sylabs/singularity/src/pkg/util/exec"
	"github.com/sylabs/singularity/src/runtime/engines/config"
	"github.com/sylabs/singularity/src/runtime/engines/kube"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type container struct {
	id         string
	podID      string
	config     *v1alpha2.ContainerConfig
	createdAt  int64
	startedAt  int64
	finishedAt int64
	state      v1alpha2.ContainerState
	imageID    string
	cmd        *exec.Cmd
}

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *v1alpha2.CreateContainerRequest) (*v1alpha2.CreateContainerResponse, error) {
	meta := req.Config.Metadata
	containerID := fmt.Sprintf("%s_%s_%d", req.PodSandboxId, meta.Name, meta.Attempt)
	originalRef := req.Config.Image.Image
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
	log.Println("will create container now")
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("could not create conatiner: %v", err)
	}
	time.Sleep(time.Second * 5)
	log.Println("container created")

	req.Config.Image.Image = originalRef
	cont := container{
		id:        containerID,
		podID:     req.GetPodSandboxId(),
		config:    req.GetConfig(),
		createdAt: time.Now().Unix(),
		state:     v1alpha2.ContainerState_CONTAINER_CREATED,
		imageID:   s.registry.ImageID(originalRef),
		cmd:       cmd,
	}

	s.pMu.RLock()
	pod := s.pods[req.PodSandboxId]
	s.pMu.RUnlock()
	pod.containers = addElem(pod.containers, containerID)
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

	s.cMu.Lock()
	defer s.cMu.Unlock()

	contI, err := kube.GetInstance(cont.id)
	if err != nil {
		return nil, fmt.Errorf("could not read container instance file: %v", err)
	}

	err = syscall.Kill(contI.Pid, syscall.SIGCONT)
	if err != nil {
		return nil, fmt.Errorf("could not start container: %v", err)
	}

	err = cont.cmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("could not wait container cmd: %v", err)
	}
	log.Printf("pid = %d", cont.cmd.ProcessState.Pid())
	cont.cmd = nil
	cont.state = v1alpha2.ContainerState_CONTAINER_RUNNING
	cont.startedAt = time.Now().Unix()
	s.containers[cont.id] = cont
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
	if cont.state == v1alpha2.ContainerState_CONTAINER_EXITED {
		return &v1alpha2.StopContainerResponse{}, nil
	}

	s.cMu.Lock()
	defer s.cMu.Unlock()

	err := killInstance(cont.id, syscall.SIGTERM)
	if err != nil {
		return nil, fmt.Errorf("could not terminate container: %v", err)
	}

	cont.state = v1alpha2.ContainerState_CONTAINER_EXITED
	cont.finishedAt = time.Now().Unix()
	s.containers[cont.id] = cont

	return &v1alpha2.StopContainerResponse{}, nil
}

// RemoveContainer removes the container. If the container is running,
// the container must be forcibly removed. This call is idempotent, and
// must not return an error if the container has already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *v1alpha2.RemoveContainerRequest) (*v1alpha2.RemoveContainerResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return &v1alpha2.RemoveContainerResponse{}, nil
	}

	s.cMu.Lock()
	defer s.cMu.Unlock()

	err := killInstance(cont.id, syscall.SIGKILL)
	if err != nil {
		return nil, fmt.Errorf("could not fill container: %v", err)
	}

	s.pMu.Lock()
	pod := s.pods[cont.podID]
	pod.containers = removeElem(pod.containers, cont.id)
	s.pods[cont.podID] = pod
	s.pMu.Unlock()

	delete(s.containers, cont.id)
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
			Metadata:    cont.config.GetMetadata(),
			State:       cont.state,
			CreatedAt:   cont.createdAt,
			StartedAt:   cont.startedAt,
			FinishedAt:  cont.finishedAt,
			ExitCode:    0,
			Image:       cont.config.GetImage(),
			ImageRef:    cont.imageID,
			Reason:      "",
			Message:     "",
			Labels:      cont.config.GetLabels(),
			Annotations: cont.config.GetAnnotations(),
			Mounts:      cont.config.GetMounts(),
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
				Metadata:     cont.config.GetMetadata(),
				Image:        cont.config.GetImage(),
				ImageRef:     cont.imageID,
				State:        cont.state,
				CreatedAt:    cont.createdAt,
				Labels:       cont.config.GetLabels(),
				Annotations:  cont.config.GetAnnotations(),
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
