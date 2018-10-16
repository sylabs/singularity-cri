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
	"path/filepath"
	"syscall"

	"net"

	"github.com/sylabs/singularity/src/pkg/sylog"
	syexec "github.com/sylabs/singularity/src/pkg/util/exec"
	"github.com/sylabs/singularity/src/runtime/engines/config"
	"github.com/sylabs/singularity/src/runtime/engines/kube"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type container struct {
	id      string
	podID   string
	imageID string
	logPath string
	conn    net.Conn

	config *k8s.ContainerConfig
	cmd    *exec.Cmd
}

// CreateContainer creates a new container in specified PodSandbox.
func (s *SingularityRuntime) CreateContainer(_ context.Context, req *k8s.CreateContainerRequest) (*k8s.CreateContainerResponse, error) {
	// hack because of SESSIONDIR in vendor
	type engineConfig struct {
		CreateContainerRequest *k8s.CreateContainerRequest
		Socket                 int
	}
	meta := req.Config.Metadata
	containerID := fmt.Sprintf("%s_%d", meta.Name, meta.Attempt)
	originalRef := req.Config.Image.Image
	req.Config.Image.Image = s.registry.ImagePath(req.Config.Image.Image) // a hack for starter to work correctly

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("could not create socket pair: %v", err)
	}

	comm := os.NewFile(uintptr(fds[0]), "")
	socket, err := net.FileConn(comm)
	if err != nil {
		return nil, fmt.Errorf("could not create socket connection: %v", err)
	}
	if err := comm.Close(); err != nil {
		log.Printf("could not close socket file: %v", err)
	}

	engineConf := config.Common{
		EngineName: "kube_container",
		EngineConfig: &engineConfig{
			CreateContainerRequest: req,
			Socket:                 fds[1],
		},
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

	cmd := exec.Command(s.starter, containerID)
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cleanup := func() {
		if err := kube.CleanupInstance(containerID); err != nil {
			log.Printf("could not cleanup %s: %v", containerID, err)
		}
		if err := socket.Close(); err != nil {
			log.Printf("could not close socket: %v", err)
		}
		if err := wait(cmd); err != nil {
			log.Printf("could not wait cmd: %v", err)
		}
	}

	if err := cmd.Start(); err != nil {
		cleanup()
		return nil, fmt.Errorf("could not schedule conatiner creation: %v", err)
	}

	data := make([]byte, 1)
	_, err = socket.Read(data)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("read socket failed: %v", err)
	}

	if data[0] == 1 {
		log.Printf("conainter created!")
	} else {
		reason := make([]byte, 1024)
		_, err = socket.Read(reason)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("read reason failed: %v", err)
		}
		cleanup()
		return nil, fmt.Errorf("conainter creation failed: %s", reason)
	}

	req.Config.Image.Image = originalRef
	logPath := req.GetSandboxConfig().GetLogDirectory()
	if logPath != "" {
		logPath = filepath.Join(logPath, req.GetConfig().GetLogPath())
	}

	cont := container{
		id:      containerID,
		podID:   req.GetPodSandboxId(),
		config:  req.GetConfig(),
		imageID: s.registry.ImageID(originalRef),
		cmd:     cmd,
		conn:    socket,
		logPath: logPath,
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

	return &k8s.CreateContainerResponse{
		ContainerId: containerID,
	}, nil
}

// StartContainer starts the container.
func (s *SingularityRuntime) StartContainer(_ context.Context, req *k8s.StartContainerRequest) (*k8s.StartContainerResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	_, err := cont.conn.Write([]byte{1})
	if err != nil {
		return nil, fmt.Errorf("could not write socket: %v", err)
	}
	if err := cont.conn.Close(); err != nil {
		return nil, fmt.Errorf("could not close socket: %v", err)
	}

	err = wait(cont.cmd)
	if err != nil {
		return nil, fmt.Errorf("could not start container: %v", err)
	}

	cont.cmd = nil
	cont.conn = nil

	s.cMu.Lock()
	s.containers[cont.id] = cont
	s.cMu.Unlock()

	return &k8s.StartContainerResponse{}, nil
}

// StopContainer stops a running container with a grace period (i.e., timeout).
// This call is idempotent, and must not return an error if the container has
// already been stopped.
// TODO: what must the runtime do after the grace period is reached?
func (s *SingularityRuntime) StopContainer(_ context.Context, req *k8s.StopContainerRequest) (*k8s.StopContainerResponse, error) {
	return &k8s.StopContainerResponse{}, s.stopContainer(req.ContainerId)
}

// RemoveContainer removes the container. If the container is running,
// the container must be forcibly removed. This call is idempotent, and
// must not return an error if the container has already been removed.
func (s *SingularityRuntime) RemoveContainer(_ context.Context, req *k8s.RemoveContainerRequest) (*k8s.RemoveContainerResponse, error) {
	return &k8s.RemoveContainerResponse{}, s.removeContainer(req.ContainerId)
}

// ContainerStatus returns status of the container.
// If the container is not present, returns an error.
func (s *SingularityRuntime) ContainerStatus(_ context.Context, req *k8s.ContainerStatusRequest) (*k8s.ContainerStatusResponse, error) {
	s.cMu.RLock()
	cont, ok := s.containers[req.ContainerId]
	s.cMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	info, err := kube.GetInfo(cont.id)
	if err != nil {
		return nil, fmt.Errorf("could not get container info: %v", err)
	}

	state := containerState(info)
	return &k8s.ContainerStatusResponse{
		Status: &k8s.ContainerStatus{
			Id:          req.ContainerId,
			Metadata:    cont.config.GetMetadata(),
			State:       state,
			CreatedAt:   info.CreatedAt,
			StartedAt:   info.StartedAt,
			FinishedAt:  info.FinishedAt,
			ExitCode:    int32(info.ExitCode),
			Image:       cont.config.GetImage(),
			ImageRef:    cont.imageID,
			Reason:      "",
			Message:     "",
			Labels:      cont.config.GetLabels(),
			Annotations: cont.config.GetAnnotations(),
			Mounts:      cont.config.GetMounts(),
			LogPath:     cont.logPath,
		},
	}, nil
}

// ListContainers lists all containers by filters.
func (s *SingularityRuntime) ListContainers(_ context.Context, req *k8s.ListContainersRequest) (*k8s.ListContainersResponse, error) {
	resp := &k8s.ListContainersResponse{}
	s.cMu.RLock()
	defer s.cMu.RUnlock()
	for _, cont := range s.containers {
		info, err := kube.GetInfo(cont.id)
		if err != nil {
			return nil, fmt.Errorf("could not get container info: %v", err)
		}
		state := containerState(info)
		if containerMatches(cont, state, req.Filter) {
			resp.Containers = append(resp.Containers, &k8s.Container{
				Id:           cont.id,
				PodSandboxId: cont.podID,
				Metadata:     cont.config.GetMetadata(),
				Image:        cont.config.GetImage(),
				ImageRef:     cont.imageID,
				State:        state,
				CreatedAt:    info.CreatedAt,
				Labels:       cont.config.GetLabels(),
				Annotations:  cont.config.GetAnnotations(),
			})
		}
	}
	return resp, nil
}

func (s *SingularityRuntime) stopContainer(containerID string) error {
	s.cMu.RLock()
	cont, ok := s.containers[containerID]
	s.cMu.RUnlock()
	if !ok {
		return fmt.Errorf("not found")
	}

	info, err := kube.GetInfo(cont.id)
	if err != nil {
		return fmt.Errorf("could not get container info: %v", err)
	}
	if info.FinishedAt != 0 {
		return nil
	}

	if err = killInstance(cont.id, syscall.SIGTERM); err != nil {
		return fmt.Errorf("could not terminate container: %v", err)
	}
	return nil
}

func (s *SingularityRuntime) removeContainer(containerID string) error {
	s.cMu.RLock()
	cont, ok := s.containers[containerID]
	s.cMu.RUnlock()
	if !ok {
		return nil
	}

	if err := killInstance(cont.id, syscall.SIGKILL); err != nil {
		return fmt.Errorf("could not kill container: %v", err)
	}
	err := wait(cont.cmd)
	if err != nil {
		return fmt.Errorf("could not wait container after kill: %v", err)
	}
	if err := kube.CleanupInstance(cont.id); err != nil {
		log.Printf("could not cleanup %s: %v", cont.id, err)
	}
	if cont.conn != nil {
		if err := cont.conn.Close(); err != nil {
			log.Printf("could not close socket: %v", err)
		}
	}

	s.pMu.Lock()
	pod := s.pods[cont.podID]
	pod.containers = removeElem(pod.containers, cont.id)
	s.pods[cont.podID] = pod
	s.pMu.Unlock()

	s.cMu.Lock()
	delete(s.containers, cont.id)
	s.cMu.Unlock()

	return nil
}

func containerState(info *kube.Info) k8s.ContainerState {
	state := k8s.ContainerState_CONTAINER_UNKNOWN
	if info.CreatedAt != 0 {
		state = k8s.ContainerState_CONTAINER_CREATED
	}
	if info.StartedAt != 0 {
		state = k8s.ContainerState_CONTAINER_RUNNING
	}
	if info.FinishedAt != 0 {
		state = k8s.ContainerState_CONTAINER_EXITED
	}
	return state
}

func containerMatches(cont container, state k8s.ContainerState, filter *k8s.ContainerFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != cont.id {
		return false
	}
	if filter.PodSandboxId != "" && filter.PodSandboxId != cont.podID {
		return false
	}
	if filter.State != nil && filter.State.State != state {
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
