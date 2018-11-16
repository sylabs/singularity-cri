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

	c, err := s.runtime.containers.Find(containerID)
	if err != nil {
		return fmt.Errorf("could not fetch container: %v", err)
	}

	if err := c.UpdateState(); err != nil {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if c.State() != k8s.ContainerState_CONTAINER_CREATED && c.State() != k8s.ContainerState_CONTAINER_RUNNING {
		return fmt.Errorf("container is not created or running")
	}

	socket := c.AttachSocket()
	if socket == "" {
		return fmt.Errorf("container didn't provide attach socket: %v", err)
	}

	/*
		kubecontainer.HandleResizing(resize, func(size remotecommand.TerminalSize) {
			logrus.Debugf("Got a resize event: %+v", size)
			_, err := fmt.Fprintf(controlFile, "%d %d %d\n", 1, size.Height, size.Width)
			if err != nil {
				logrus.Debugf("Failed to write to control file to resize terminal: %v", err)
			}
		})

		attachSocketPath := filepath.Join(ss.runtimeServer.Config().ContainerAttachSocketDir, c.ID(), "attach")
		conn, err := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: attachSocketPath, Net: "unixpacket"})
		if err != nil {
			return fmt.Errorf("failed to connect to container %s attach socket: %v", c.ID(), err)
		}
		defer conn.Close()

		receiveStdout := make(chan error)
		if outputStream != nil || errorStream != nil {
			go func() {
				receiveStdout <- redirectResponseToOutputStreams(outputStream, errorStream, conn)
			}()
		}

		stdinDone := make(chan error)
		go func() {
			var err error
			if inputStream != nil {
				_, err = utils.CopyDetachable(conn, inputStream, nil)
				conn.CloseWrite()
			}
			stdinDone <- err
		}()

		select {
		case err := <-receiveStdout:
			return err
		case err := <-stdinDone:
			if _, ok := err.(utils.DetachError); ok {
				return nil
			}
			if outputStream != nil || errorStream != nil {
				return <-receiveStdout
			}
		}

		return nil
	*/
	return fmt.Errorf("not implemented")
}

// PortForward ...
func (s *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	return fmt.Errorf("not implemented")
}
