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

var (
	// ErrContainerNotCreated is used when attempting to perform operations on containers that
	// are not in CONTAINER_CREATED state, e.g. start already started container.
	ErrContainerNotCreated = fmt.Errorf("container is not in %s state", k8s.ContainerState_CONTAINER_CREATED.String())
)

// Container represents kubernetes container inside a pod. It encapsulates
// all container-specific logic and should be used by runtime for correct interaction.
type Container struct {
	id string
	*k8s.ContainerConfig
	pod *Pod

	runtimeState runtime.State
	createdAt    int64 // unix nano
	startedAt    int64 // unix nano
	finishedAt   int64 // unix nano
	exitDesc     string
	exitCode     int32

	attachSocket  string
	controlSocket string
	logPath       string

	createOnce sync.Once
	isStopped  bool
	isRemoved  bool

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

// State returns current container state understood by k8s.
func (c *Container) State() k8s.ContainerState {
	switch c.runtimeState {
	case runtime.StateCreated:
		return k8s.ContainerState_CONTAINER_CREATED
	case runtime.StateRunning:
		return k8s.ContainerState_CONTAINER_RUNNING
	case runtime.StateExited:
		return k8s.ContainerState_CONTAINER_EXITED
	}
	return k8s.ContainerState_CONTAINER_UNKNOWN

}

// CreatedAt returns pod creation time in Unix nano.
func (c *Container) CreatedAt() int64 {
	return c.createdAt
}

// StartedAt returns container start time in unix nano.
func (c *Container) StartedAt() int64 {
	return c.startedAt
}

// FinishedAt returns container finish time in unix nano.
func (c *Container) FinishedAt() int64 {
	return c.finishedAt
}

// ExitCode returns container ext code.
func (c *Container) ExitCode() int32 {
	return c.exitCode
}

// ExitDescription returns human readable message of why container has exited.
func (c *Container) ExitDescription() string {
	return c.exitDesc
}

// AttachSocket returns attach socket on which runtime will serve attach request.
func (c *Container) AttachSocket() string {
	return c.attachSocket
}

// ControlSocket returns control socket on which runtime will wait for
// control signals, e.g. resize event.
func (c *Container) ControlSocket() string {
	return c.controlSocket
}

// LogPath returns and absolute path to container logs on the host
// filesystem or empty string if logs are not collected.
func (c *Container) LogPath() string {
	return c.logPath
}

// Create creates container inside a pod from the image.
func (c *Container) Create(info *image.Info) error {
	var err error
	defer func() {
		if err != nil {
			if err := c.kill(); err != nil {
				log.Printf("could not kill container failed run: %v", err)
			}
			if err := c.cli.Delete(c.id); err != nil {
				log.Printf("could not delete container: %v", err)
			}
			if err := c.cleanupFiles(true); err != nil {
				log.Printf("could not cleanup bundle: %v", err)
			}
		}
	}()

	c.createOnce.Do(func() {
		err = c.addLogDirectory()
		if err != nil {
			err = fmt.Errorf("could not create log directory: %v", err)
			return
		}
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
	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if c.State() != k8s.ContainerState_CONTAINER_CREATED {
		return ErrContainerNotCreated
	}
	go c.cli.Start(c.id)
	err := c.expectState(runtime.StateRunning)
	if err != nil {
		return err
	}
	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	return nil
}

// Stop stops running container. The passed timeout is used to give
// container a chance to stop gracefully. If timeout is 0 or container
// is still running after grace period, it will be forcibly terminated.
func (c *Container) Stop(timeout int64) error {
	if c.isStopped {
		return nil
	}

	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if err := c.terminate(timeout); err != nil {
		return fmt.Errorf("could not terminate container process: %v", err)
	}
	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	c.isStopped = true
	return nil
}

// Remove removes the container, making sure nothing
// of it left on the host filesystem. When no Stop is called before
// Remove forcibly kills container process.
func (c *Container) Remove() error {
	if c.isRemoved {
		return nil
	}

	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if err := c.kill(); err != nil {
		log.Printf("could not kill container failed run: %v", err)
	}
	if err := c.cli.Delete(c.id); err != nil {
		log.Printf("could not delete container: %v", err)
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

	if filter.State != nil && filter.State.State != c.State() {
		return false
	}

	for k, v := range filter.LabelSelector {
		label, ok := c.Labels[k]
		if !ok {
			return false
		}
		if v != label {
			return false
		}
	}
	return true
}
