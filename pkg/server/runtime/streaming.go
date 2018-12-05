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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

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

// Exec executes a command inside a container with attaching passed io streams to it.
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

	var execErr error
	if tty {
		// stderr is nil here
		execCmd := c.PrepareExec(cmd)

		master, err := pty.Start(execCmd)
		if err != nil {
			return fmt.Errorf("could not start exec in pty: %v", err)
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
		defer close(done)

		if stdin != nil {
			go io.Copy(master, stdin)
		}
		if stdout != nil {
			go io.Copy(stdout, master)
		}
		execErr = execCmd.Wait()
	} else {
		execErr = c.Exec(cmd, stdin, stdout, stderr)
	}

	log.Printf("Exec for %s returned %v...", containerID, execErr)
	return execErr
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

// PortForward enters pod's NET namespace to forward passed
// stream to the given port and back.
func (s *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	p, err := s.runtime.pods.Find(podSandboxID)
	if err != nil {
		return fmt.Errorf("could not fetch container: %v", err)
	}

	if err := p.UpdateState(); err != nil {
		return fmt.Errorf("could not update pod state: %v", err)
	}
	if p.State() != k8s.PodSandboxState_SANDBOX_READY {
		return fmt.Errorf("pod is not ready")
	}

	socatPath, err := exec.LookPath("socat")
	if err != nil {
		return fmt.Errorf("unable to do port forwarding: socat not found")
	}
	nsenterPath, err := exec.LookPath("nsenter")
	if err != nil {
		return fmt.Errorf("unable to do port forwarding: nsenter not found")
	}

	args := []string{"-t", fmt.Sprintf("%d", p.Pid()), "-n", socatPath, "-", fmt.Sprintf("TCP4:localhost:%d", port)}
	commandString := fmt.Sprintf("%s %s", nsenterPath, strings.Join(args, " "))
	log.Printf("executing port forwarding command: %s", commandString)

	var stderr bytes.Buffer
	cmd := exec.Command(nsenterPath, args...)
	cmd.Stdout = stream
	cmd.Stderr = &stderr

	// If we use Stdin, cmd.Run() won't return until the goroutine that's copying
	// from stream finishes. Unfortunately, if you have a client like telnet connected
	// via port forwarding, as long as the user's telnet client is connected to the user's
	// local listener that port forwarding sets up, the telnet session never exits. This
	// means that even if socat has finished running, cmd.Run() won't ever return
	// (because the client still has the connection and stream open).
	//
	// The work around is to use StdinPipe(), as Wait() (called by Run()) closes the pipe
	// when the cmd (socat) exits.
	inPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("unable to do port forwarding: error creating stdin pipe: %v", err)
	}
	go func() {
		io.Copy(inPipe, stream)
		inPipe.Close()
	}()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.Bytes())
	}

	return nil
}
