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
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"k8s.io/client-go/tools/remotecommand"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type streamingRuntime struct {
	runtime *SingularityRuntime
}

// Exec ...
func (s *streamingRuntime) Exec(containerID string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	return fmt.Errorf("not implemented")
}

// Attach attaches passed streams to the container.
func (s *streamingRuntime) Attach(containerID string,
	stdin io.Reader, stdout, stderr io.WriteCloser,
	tty bool, resize <-chan remotecommand.TerminalSize) error {

	log.Printf("Attaching to %s...", containerID)
	c, err := s.runtime.containers.Find(containerID)
	if err != nil {
		return fmt.Errorf("could not fetch container: %v", err)
	}

	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if c.State() != k8s.ContainerState_CONTAINER_RUNNING {
		return fmt.Errorf("container is not running")
	}

	socket := c.AttachSocket()
	if socket == "" {
		return fmt.Errorf("container didn't provide attach socket: %v", err)
	}

	log.Printf("Dialing %s...", socket)
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return fmt.Errorf("could not conntect to attach socket: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to attach socket %s...", socket)
	go func() {
		for size := range resize {
			log.Printf("got resize event for %s: %+v", containerID, size)
		}
	}()

	errors := make(chan error, 3)
	var wg sync.WaitGroup

	if stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := io.Copy(stdout, conn)
			errors <- err
		}()
	}
	if stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := io.Copy(stderr, conn)
			errors <- err
		}()
	}
	if stdin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := io.Copy(conn, stdin)
			errors <- err
		}()
	}

	err = <-errors

	log.Printf("Waiting attach end %s...", containerID)
	wg.Wait()
	log.Printf("Attach for %s returved %v...", containerID, err)
	return err
}

// PortForward ...
func (s *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	return fmt.Errorf("not implemented")
}
