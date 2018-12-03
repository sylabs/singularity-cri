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
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/kr/pty"
	"github.com/kubernetes-sigs/cri-o/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/singularity/pkg/ociruntime"
	"github.com/sylabs/singularity/pkg/util/unix"
	"k8s.io/client-go/tools/remotecommand"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type streamingRuntime struct {
	runtime *SingularityRuntime
}

func (s *streamingRuntime) Exec(containerID string, cmd []string,
	stdin io.Reader, stdout, stderr io.WriteCloser,
	tty bool, resize <-chan remotecommand.TerminalSize) error {

	log.Printf("Exec %v in %s...", cmd, containerID)
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

	if tty {
		// stderr is nil here
		master, slave, err := pty.Open()
		if err != nil {
			return fmt.Errorf("could not open pts: %v", err)
		}

		done := make(chan struct{})
		go func() {
			log.Printf("resize start for %s", containerID)
			for {
				select {
				case <-done:
					log.Printf("resize end for %s", containerID)
					return
				case size := <-resize:
					log.Printf("got resize event for %s: %+v", containerID, size)
					s := &pty.Winsize{
						Cols: uint16(size.Width),
						Rows: uint16(size.Height),
					}
					if err := pty.Setsize(master, s); err != nil {
						log.Printf("could not resize terminal: %v", err)
					}
				}
			}
		}()

		defer master.Close()
		defer slave.Close()
		defer close(done)

		if stdin != nil {
			go io.Copy(master, stdin)

		}
		if stdout != nil {
			go io.Copy(stdout, master)
		}

		err = c.Exec(cmd, slave, slave, slave)
	} else {
		err = c.Exec(cmd, stdin, stdout, stderr)
	}

	log.Printf("Exec for %s returned %v...", containerID, err)

	return err
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
	attachSock, err := unix.Dial(socket)
	if err != nil {
		return fmt.Errorf("could not conntect to attach socket: %v", err)
	}
	defer attachSock.Close()

	done := make(chan struct{})
	go func() {
		socket := c.ControlSocket()
		if socket == "" {
			log.Printf("container didn't provide control socket: %v", err)
			return
		}

		log.Printf("resize start for %s", containerID)
		for {
			select {
			case <-done:
				log.Printf("resize end for %s", containerID)
				return
			case size := <-resize:
				log.Printf("got resize event for %s: %+v", containerID, size)
				ctrlSock, err := unix.Dial(socket)
				if err != nil {
					log.Printf("could not conntect to control socket: %v", err)
					continue
				}
				ctrl := ociruntime.Control{
					ConsoleSize: &specs.Box{
						Height: uint(size.Height),
						Width:  uint(size.Width),
					},
				}
				err = json.NewEncoder(ctrlSock).Encode(&ctrl)
				if err != nil {
					log.Printf("could not send resize event to control socket: %v", err)
				}
				ctrlSock.Close()
			}
		}
	}()

	errors := make(chan error, 2)
	if stdout != nil || stderr != nil {
		go func() {
			out := stdout
			if out == nil {
				out = stderr
			}

			_, err := io.Copy(out, attachSock)
			log.Printf("stdout/stderr got error %v", err)
			errors <- err
		}()
	}
	if tty && c.GetStdin() && stdin != nil {
		go func() {
			// copy until ctrl-d hits
			_, err := utils.CopyDetachable(attachSock, stdin, []byte{4})
			log.Printf("stdin got error %v", err)
			errors <- err
		}()
	}

	err = <-errors
	close(done)
	log.Printf("Attach for %s returned %v...", containerID, err)
	if (err == utils.DetachError{}) {
		return nil
	}
	return err
}

// PortForward ...
func (s *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	return fmt.Errorf("not implemented")
}
