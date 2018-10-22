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
	"os"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type pod struct {
	id         string
	config     *k8s.PodSandboxConfig
	state      k8s.PodSandboxState
	createdAt  int64 // unix nano
	namespaces []specs.LinuxNamespace
	containers []string
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes must ensure
// the sandbox is in the ready state on success.
func (s *SingularityRuntime) RunPodSandbox(_ context.Context, req *k8s.RunPodSandboxRequest) (r *k8s.RunPodSandboxResponse, err error) {
	pod := &pod{
		id:     podID(req.GetConfig().GetMetadata()),
		config: req.GetConfig(),
	}
	defer func() {
		if err != nil {
			cleanupPod(pod, true)
		}
	}()

	if err := ensurePodDirectories(pod.id); err != nil {
		return nil, status.Errorf(codes.Internal, "could not ensure pod directories: %v", err)
	}

	if pod.config.GetLogDirectory() != "" {
		log.Printf("creating log directory %s", pod.config.GetLogDirectory())
		err := os.MkdirAll(pod.config.GetLogDirectory(), os.ModePerm)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not create log directory: %v", err)
		}
	}

	if pod.config.GetDnsConfig() != nil {
		err := addResolvConf(pod.id, pod.config.GetDnsConfig())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not configure dns: %v", err)
		}
	}

	if pod.config.GetHostname() != "" {
		err := addHostname(pod.id, pod.config.GetHostname())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not configure hostname: %v", err)
		}
		utsPath := bindNamespacePath(pod.id, specs.UTSNamespace)
		log.Printf("unsharing uts namespace at %s", utsPath)
		pod.namespaces = append(pod.namespaces, specs.LinuxNamespace{
			Type: specs.UTSNamespace,
			Path: utsPath,
		})
	}
	security := pod.config.GetLinux().GetSecurityContext()
	if security.GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD {
		netPath := bindNamespacePath(pod.id, specs.NetworkNamespace)
		log.Printf("unsharing net namespace at %s", netPath)
		pod.namespaces = append(pod.namespaces, specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: netPath,
		})
	}
	if security.GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD {
		ipcPath := bindNamespacePath(pod.id, specs.IPCNamespace)
		log.Printf("unsharing ipc namespace at %s", ipcPath)
		pod.namespaces = append(pod.namespaces, specs.LinuxNamespace{
			Type: specs.IPCNamespace,
			Path: ipcPath,
		})
	}
	if err := namespace.UnshareAll(pod.namespaces); err != nil {
		return nil, status.Errorf(codes.Internal, "could not unshare namespaces: %v", err)
	}

	pod.state = k8s.PodSandboxState_SANDBOX_READY
	pod.createdAt = time.Now().UnixNano()
	s.pMu.Lock()
	s.pods[pod.id] = pod
	s.pMu.Unlock()

	return &k8s.RunPodSandboxResponse{
		PodSandboxId: pod.id,
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
func (s *SingularityRuntime) StopPodSandbox(_ context.Context, req *k8s.StopPodSandboxRequest) (*k8s.StopPodSandboxResponse, error) {
	pod := s.findPod(req.PodSandboxId)
	if pod == nil {
		return nil, status.Error(codes.NotFound, "pod not found")
	}
	if pod.state == k8s.PodSandboxState_SANDBOX_NOTREADY {
		return &k8s.StopPodSandboxResponse{}, nil
	}

	for _, contID := range pod.containers {
		log.Printf("stopping container %s", contID)
		// todo stop container
	}

	// todo reclaim resources somewhere here

	pod.state = k8s.PodSandboxState_SANDBOX_NOTREADY
	s.pMu.Lock()
	s.pods[pod.id] = pod
	s.pMu.Unlock()
	return &k8s.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox removes the sandbox. If there are any running containers
// in the sandbox, they must be forcibly terminated and removed.
// This call is idempotent, and must not return an error if the sandbox has
// already been removed.
func (s *SingularityRuntime) RemovePodSandbox(_ context.Context, req *k8s.RemovePodSandboxRequest) (*k8s.RemovePodSandboxResponse, error) {
	pod := s.findPod(req.PodSandboxId)
	if pod == nil {
		return &k8s.RemovePodSandboxResponse{}, nil
	}

	for _, contID := range pod.containers {
		log.Printf("removing container %s", contID)
		// todo remove container
	}

	if err := cleanupPod(pod, false); err != nil {
		return nil, status.Errorf(codes.Internal, "could not cleanup pod: %v", err)
	}

	s.pMu.Lock()
	delete(s.pods, pod.id)
	s.pMu.Unlock()
	return &k8s.RemovePodSandboxResponse{}, nil
}

// PodSandboxStatus returns the status of the PodSandbox.
// If the PodSandbox is not present, returns an error.
func (s *SingularityRuntime) PodSandboxStatus(_ context.Context, req *k8s.PodSandboxStatusRequest) (*k8s.PodSandboxStatusResponse, error) {
	pod := s.findPod(req.PodSandboxId)
	if pod == nil {
		return nil, status.Error(codes.NotFound, "pod not found")
	}

	return &k8s.PodSandboxStatusResponse{
		Status: &k8s.PodSandboxStatus{
			Id:        pod.id,
			Metadata:  pod.config.GetMetadata(),
			State:     pod.state,
			CreatedAt: pod.createdAt,
			Network:   nil, // todo later
			Linux: &k8s.LinuxPodSandboxStatus{
				Namespaces: &k8s.Namespace{
					Options: pod.config.GetLinux().GetSecurityContext().GetNamespaceOptions(),
				},
			},
			Labels:      pod.config.GetLabels(),
			Annotations: pod.config.GetAnnotations(),
		},
	}, nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (s *SingularityRuntime) ListPodSandbox(_ context.Context, req *k8s.ListPodSandboxRequest) (*k8s.ListPodSandboxResponse, error) {
	var pods []*k8s.PodSandbox

	s.pMu.RLock()
	defer s.pMu.RUnlock()
	for _, pod := range s.pods {
		if podMatches(pod, req.Filter) {
			pods = append(pods, &k8s.PodSandbox{
				Id:          pod.id,
				Metadata:    pod.config.GetMetadata(),
				State:       pod.state,
				CreatedAt:   pod.createdAt,
				Labels:      pod.config.GetLabels(),
				Annotations: pod.config.GetAnnotations(),
			})
		}
	}
	return &k8s.ListPodSandboxResponse{
		Items: pods,
	}, nil
}

func (s *SingularityRuntime) findPod(id string) *pod {
	s.pMu.RLock()
	defer s.pMu.RUnlock()
	return s.pods[id]
}
