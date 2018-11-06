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

package kube

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/rand"
	"github.com/sylabs/cri/pkg/singularity/runtime"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	// ContainerIDLen reflects number of symbols in container unique ID.
	ContainerIDLen = 64
)

// Container represents kubernetes container inside a pod. It encapsulates
// all container-specific logic and should be used by runtime for correct interaction.
type Container struct {
	id string
	*k8s.ContainerConfig
	pod *Pod

	createdAt    int64 // unix nano
	startedAt    int64 // unix nano
	finishedAt   int64 // unix nano
	exitCode     int32
	state        k8s.ContainerState
	runtimeState runtime.State

	createOnce  sync.Once
	startedOnce sync.Once
	isStopped   bool
	isRemoved   bool

	cli        *runtime.CLIClient
	syncChan   <-chan runtime.State
	syncCancel context.CancelFunc
}

// NewContainer constructs Container instance. Container is thread safe to use.
func NewContainer(config *k8s.ContainerConfig, pod *Pod) *Container {
	contID := rand.GenerateID(ContainerIDLen)
	return &Container{
		id:              contID,
		ContainerConfig: config,
		pod:             pod,
		cli:             runtime.NewCLIClient(),
	}
}

// ID returns unique container ID.
func (c *Container) ID() string {
	return c.id
}

// PodID returns ID of a pod container is executed in.
func (c *Container) PodID() string {
	return c.pod.ID()
}

// State returns current pod state.
func (c *Container) State() k8s.ContainerState {
	return c.state
}

// CreatedAt returns pod creation time in Unix nano.
func (c *Container) CreatedAt() int64 {
	return c.createdAt
}

func (c *Container) StartedAt() int64 {
	return c.startedAt
}

func (c *Container) FinishedAt() int64 {
	return c.finishedAt
}

func (c *Container) ExitCode() int32 {
	return c.exitCode
}

// Create creates container inside a pod from the image.
func (c *Container) Create(info *image.Info) error {
	var err error
	defer func() {
		if err != nil {
			if err := c.cleanupRuntime(true); err != nil {
				log.Printf("could not kill container failed run: %v", err)
			}
			if err := c.cleanupFiles(true); err != nil {
				log.Printf("could not cleanup bundle: %v", err)
			}
		}
	}()

	c.createOnce.Do(func() {
		err = c.spawnOCIContainer(info)
		if err != nil {
			err = fmt.Errorf("could not spawn container: %v", err)
			return
		}
		err = c.UpdateState()
		if err != nil {
			err = fmt.Errorf("could not update container state: %v", err)
			return
		}
		c.pod.addContainer(c)
	})
	return err
}

// Start starts created container.
func (c *Container) Start() error {
	var err error

	c.startedOnce.Do(func() {
		go c.cli.Start(c.id)
		err = c.expectState(runtime.StateRunning)
		if err != nil {
			return
		}
		err = c.UpdateState()
		if err != nil {
			err = fmt.Errorf("could not update container state: %v", err)
			return
		}
	})
	return err
}

// Stop stops running container.
func (c *Container) Stop() error {
	if c.isStopped {
		return nil
	}

	go c.cli.Kill(c.id, false)
	err := c.expectState(runtime.StateExited)
	if err != nil {
		return fmt.Errorf("could not stop container: %v", err)
	}
	err = c.UpdateState()
	if err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	c.isStopped = true
	return err
}

// Remove removes the container, making sure nothing
// of it left on the host filesystem. When no Stop is called before
// Remove forcibly kills container process.
func (c *Container) Remove() error {
	if c.isRemoved {
		return nil
	}
	if err := c.cleanupRuntime(true); err != nil {
		return fmt.Errorf("could not kill container process: %v", err)
	}
	if err := c.cleanupFiles(false); err != nil {
		return fmt.Errorf("could not cleanup container: %v", err)
	}
	c.pod.removeContainer(c)
	c.isRemoved = true
	return nil
}

// MatchesFilter tests Container against passed filter and returns true if it matches.
func (c *Container) MatchesFilter(filter *k8s.ContainerFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Id != "" && filter.Id != c.id {
		return false
	}

	if filter.PodSandboxId != "" && filter.PodSandboxId != c.pod.ID() {
		return false
	}

	if filter.State != nil && filter.State.State != c.state {
		return false
	}

	for k, v := range filter.LabelSelector {
		lablel, ok := c.Labels[k]
		if !ok {
			return false
		}
		if v != lablel {
			return false
		}
	}
	return true
}
