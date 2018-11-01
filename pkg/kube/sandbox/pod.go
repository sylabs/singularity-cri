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

package sandbox

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/namespace"
	"github.com/sylabs/cri/pkg/rand"
	"github.com/sylabs/cri/pkg/singularity/runtime"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	podIDLen = 64
)

// Pod represents kubernetes pod. It encapsulates all pod-specific
// logic and should be used by runtime for correct interaction.
type Pod struct {
	id string
	*k8s.PodSandboxConfig

	runOnce    sync.Once
	stopOnce   sync.Once
	removeOnce sync.Once

	state      k8s.PodSandboxState
	createdAt  int64 // unix nano
	namespaces []specs.LinuxNamespace

	cli        *runtime.CLIClient
	syncChan   <-chan runtime.State
	syncCancel context.CancelFunc
}

// New constructs Pod instance. Pod is thread safe to use.
func New(config *k8s.PodSandboxConfig) *Pod {
	podID := rand.GenerateID(podIDLen)
	return &Pod{
		PodSandboxConfig: config,
		id:               podID,
		state:            k8s.PodSandboxState_SANDBOX_NOTREADY,
		cli:              runtime.NewCLIClient(),
	}
}

// ID returns unique pod ID.
func (p *Pod) ID() string {
	return p.id
}

// Run prepares and runs pod based on initial config passed to NewPod.
func (p *Pod) Run() error {
	var err error
	defer func() {
		if err != nil {
			if err := p.cleanupRuntime(true); err != nil {
				log.Printf("could not kill pod failed run: %v", err)
			}
			if err := p.cleanupFiles(true); err != nil {
				log.Printf("could not cleanupFiles pod after failed run: %v", err)
			}
		}
	}()

	p.runOnce.Do(func() {
		if err = p.prepareFiles(); err != nil {
			err = fmt.Errorf("could not create pod directories: %v", err)
			return
		}
		if err = p.unshareNamespaces(); err != nil {
			err = fmt.Errorf("could not unshare namespaces: %v", err)
			return
		}
		if err = p.spawnOCIPod(); err != nil {
			err = fmt.Errorf("could not spawn pod: %v", err)
			return
		}
		p.state = k8s.PodSandboxState_SANDBOX_READY
		p.createdAt = time.Now().UnixNano()
	})
	return err
}

// Stop stops pod and all its containers, reclaims any resources.
func (p *Pod) Stop() error {
	if p.state == k8s.PodSandboxState_SANDBOX_NOTREADY {
		return nil
	}

	var err error
	p.stopOnce.Do(func() {
		// todo stop containers
		// todo reclaim resources somewhere here
		err = p.cleanupRuntime(false)
		if err != nil {
			err = fmt.Errorf("could not stop pod process: %v", err)
			return
		}
		p.state = k8s.PodSandboxState_SANDBOX_NOTREADY
	})
	return err
}

// Remove removes pod and all its containers, making sure nothing
// of it left on the host filesystem. When no Stop() is called before
// Remove forcibly kills all containers and pod itself.
func (p *Pod) Remove() error {
	var err error
	p.removeOnce.Do(func() {
		// todo remove containers
		err = p.cleanupRuntime(true)
		if err != nil {
			err = fmt.Errorf("could not kill pod process: %v", err)
			return
		}
		err = p.cli.Delete(p.id)
		if err != nil {
			err = fmt.Errorf("could not remove pod: %v", err)
			return
		}
		if err = p.cleanupFiles(false); err != nil {
			err = fmt.Errorf("could not cleanupFiles pod: %v", err)
		}
	})
	return err
}

// State returns current pod state.
func (p *Pod) State() k8s.PodSandboxState {
	return p.state
}

// CreatedAt returns pod creation time in Unix nano.
func (p *Pod) CreatedAt() int64 {
	return p.createdAt
}

// MatchesFilter tests Pod against passed filter and returns true if it matches.
func (p *Pod) MatchesFilter(filter *k8s.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != p.id {
		return false
	}

	if filter.State != nil && filter.State.State != p.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := p.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}

func (p *Pod) unshareNamespaces() error {
	if p.GetHostname() != "" {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.UTSNamespace,
			Path: p.bindNamespacePath(specs.UTSNamespace),
		})
	}
	security := p.GetLinux().GetSecurityContext()
	if security.GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: p.bindNamespacePath(specs.NetworkNamespace),
		})
	}
	if security.GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD {
		p.namespaces = append(p.namespaces, specs.LinuxNamespace{
			Type: specs.IPCNamespace,
			Path: p.bindNamespacePath(specs.IPCNamespace),
		})
	}
	if err := namespace.UnshareAll(p.namespaces); err != nil {
		return fmt.Errorf("unsahre all failed: %v", err)
	}
	return nil
}
