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
	"strconv"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/singularity/runtime"
)

func (c *Container) spawnOCIContainer(imgInfo *image.Info) error {
	err := c.addOCIBundle(imgInfo)
	if err != nil {
		return fmt.Errorf("could not create oci bundle: %v", err)
	}
	log.Printf("launching observe server...")
	syncCtx, cancel := context.WithCancel(context.Background())
	c.syncCancel = cancel
	c.syncChan, err = runtime.ObserveState(syncCtx, c.socketPath())
	if err != nil {
		return fmt.Errorf("could not listen for state changes: %v", err)
	}

	go c.cli.Create(c.id, c.bundlePath(), "--sync-socket", c.socketPath())

	if err := c.expectState(runtime.StateCreating); err != nil {
		return err
	}
	if err := c.expectState(runtime.StateCreated); err != nil {
		return err
	}

	return nil
}

func (c *Container) expectState(expect runtime.State) error {
	log.Printf("waiting for state %d...", expect)
	c.runtimeState = <-c.syncChan
	if c.runtimeState != expect {
		return fmt.Errorf("unexpected container state: %v", c.runtimeState)
	}
	return nil
}

func (c *Container) UpdateState() error {
	contState, err := c.cli.State(c.id)
	if err != nil {
		return fmt.Errorf("could not get container state: %v", err)
	}
	c.createdAt, err = strconv.ParseInt(contState.Annotations[runtime.AnnotationCreatedAt], 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse created timestamp: %v", err)
	}
	c.startedAt, err = strconv.ParseInt(contState.Annotations[runtime.AnnotationStartedAt], 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse started timestamp: %v", err)
	}
	c.finishedAt, err = strconv.ParseInt(contState.Annotations[runtime.AnnotationFinishedAt], 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse finished timestamp: %v", err)
	}
	exitCode, err := strconv.ParseInt(contState.Annotations[runtime.AnnotationExitCode], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse exit code: %v", err)
	}
	c.exitCode = int32(exitCode)
	c.runtimeState = runtime.StatusToState(contState.Status)

	if err != nil {
		return fmt.Errorf("could not parse annotations: %v", err)
	}
	return nil
}

func (c *Container) cleanupRuntime(force bool) error {
	if c.runtimeState == runtime.StateExited {
		return nil
	}
	err := c.cli.Kill(c.id, force)
	if err != nil {
		return fmt.Errorf("could not treminate container: %v", err)
	}
	err = c.expectState(runtime.StateExited)
	if err != nil {
		return err
	}
	c.syncCancel()
	if err := c.cli.Delete(c.id); err != nil {
		return fmt.Errorf("could not remove container: %v", err)
	}
	return nil
}
