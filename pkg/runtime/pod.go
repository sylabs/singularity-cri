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
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sylabs/singularity/src/pkg/sylog"
	syexec "github.com/sylabs/singularity/src/pkg/util/exec"
	"github.com/sylabs/singularity/src/runtime/engines/config"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type pod struct {
	id         string
	config     *v1alpha2.PodSandboxConfig
	state      v1alpha2.PodSandboxState
	createdAt  int64
	containers []string
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes must ensure
// the sandbox is in the ready state on success.
func (s *SingularityRuntime) RunPodSandbox(_ context.Context, req *v1alpha2.RunPodSandboxRequest) (*v1alpha2.RunPodSandboxResponse, error) {
	meta := req.GetConfig().GetMetadata()
	podID := fmt.Sprintf("%s_%s_%s_%d", meta.Name, meta.Namespace, meta.Uid, meta.Attempt)

	engineConf := config.Common{
		EngineName:   "kube_podsandbox",
		EngineConfig: req.GetConfig(),
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

	cmd := exec.Command("starter", podID)
	cmd.Env = envs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not start pod: %s", err)
	}

	pod := pod{
		id:        podID,
		config:    req.GetConfig(),
		state:     v1alpha2.PodSandboxState_SANDBOX_READY,
		createdAt: time.Now().Unix(),
	}
	s.pMu.Lock()
	s.pods[podID] = pod
	s.pMu.Unlock()

	return &v1alpha2.RunPodSandboxResponse{
		PodSandboxId: podID,
	}, nil
}

// StopPodSandbox stops any running process that is part of the sandbox and
// reclaims network resources (e.g., IP addresses) allocated to the sandbox.
// If there are any running containers in the sandbox, they must be forcibly
// terminated.
// This call is idempotent, and must not return an error if all relevant
// resources have already been reclaimed. kubelet will call StopPodSandbox
// at least once before calling RemovePodSandbox. It will also attempt to
// reclaim resources eagerly, as soon as a sandbox is not needed. Hence,
// multiple StopPodSandbox calls are expected.
func (s *SingularityRuntime) StopPodSandbox(_ context.Context, req *v1alpha2.StopPodSandboxRequest) (*v1alpha2.StopPodSandboxResponse, error) {
	s.pMu.RLock()
	pod, ok := s.pods[req.PodSandboxId]
	s.pMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if pod.state == v1alpha2.PodSandboxState_SANDBOX_NOTREADY {
		return &v1alpha2.StopPodSandboxResponse{}, nil
	}

	s.pMu.Lock()
	defer s.pMu.Unlock()

	// todo reclaim resources somewhere here

	for _, contID := range pod.containers {
		err := killInstance(contID, syscall.SIGTERM)
		if err != nil {
			return nil, fmt.Errorf("could not terminate container: %v", err)
		}

		s.cMu.Lock()
		cont := s.containers[contID] // assume containers are running
		cont.state = v1alpha2.ContainerState_CONTAINER_EXITED
		cont.finishedAt = time.Now().Unix()
		s.containers[contID] = cont
		s.cMu.Unlock()
	}

	err := killInstance(pod.id, syscall.SIGTERM)
	if err != nil {
		return nil, fmt.Errorf("could not terminate pod: %v", err)
	}

	pod.state = v1alpha2.PodSandboxState_SANDBOX_NOTREADY
	s.pods[pod.id] = pod

	return &v1alpha2.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox removes the sandbox. If there are any running containers
// in the sandbox, they must be forcibly terminated and removed.
// This call is idempotent, and must not return an error if the sandbox has
// already been removed.
func (s *SingularityRuntime) RemovePodSandbox(_ context.Context, req *v1alpha2.RemovePodSandboxRequest) (*v1alpha2.RemovePodSandboxResponse, error) {
	s.pMu.RLock()
	pod, ok := s.pods[req.PodSandboxId]
	s.pMu.RUnlock()
	if !ok {
		return &v1alpha2.RemovePodSandboxResponse{}, nil
	}

	s.pMu.Lock()
	defer s.pMu.Unlock()

	for _, contID := range pod.containers {
		err := killInstance(contID, syscall.SIGKILL)
		if err != nil {
			return nil, fmt.Errorf("could not kill container: %v", err)
		}
		s.cMu.Lock()
		delete(s.containers, contID)
		s.cMu.Unlock()
	}

	err := killInstance(pod.id, syscall.SIGKILL)
	if err != nil {
		return nil, fmt.Errorf("could not kill pod: %v", err)
	}
	delete(s.pods, pod.id)
	return &v1alpha2.RemovePodSandboxResponse{}, nil
}

// PodSandboxStatus returns the status of the PodSandbox.
// If the PodSandbox is not present, returns an error.
func (s *SingularityRuntime) PodSandboxStatus(_ context.Context, req *v1alpha2.PodSandboxStatusRequest) (*v1alpha2.PodSandboxStatusResponse, error) {
	s.pMu.RLock()
	pod, ok := s.pods[req.PodSandboxId]
	s.pMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	ns := pod.config.GetLinux().GetSecurityContext().GetNamespaceOptions()
	return &v1alpha2.PodSandboxStatusResponse{
		Status: &v1alpha2.PodSandboxStatus{
			Id:        pod.id,
			Metadata:  pod.config.GetMetadata(),
			State:     pod.state,
			CreatedAt: pod.createdAt,
			Network:   nil, // todo
			Linux: &v1alpha2.LinuxPodSandboxStatus{
				Namespaces: &v1alpha2.Namespace{
					Options: ns,
				},
			},
			Labels:      pod.config.GetLabels(),
			Annotations: pod.config.GetAnnotations(),
		},
	}, nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (s *SingularityRuntime) ListPodSandbox(_ context.Context, req *v1alpha2.ListPodSandboxRequest) (*v1alpha2.ListPodSandboxResponse, error) {
	resp := &v1alpha2.ListPodSandboxResponse{}
	s.pMu.RLock()
	defer s.pMu.RUnlock()
	for _, pod := range s.pods {
		if podMatches(pod, req.Filter) {
			resp.Items = append(resp.Items, &v1alpha2.PodSandbox{
				Id:          pod.id,
				Metadata:    pod.config.GetMetadata(),
				State:       pod.state,
				CreatedAt:   pod.createdAt,
				Labels:      pod.config.GetLabels(),
				Annotations: pod.config.GetAnnotations(),
			})
		}
	}
	return resp, nil
}

func podMatches(pod pod, filter *v1alpha2.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != pod.id {
		return false
	}

	if filter.State != nil && filter.State.State != pod.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := pod.config.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
