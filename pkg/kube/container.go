// Copyright (c) 2018-2019 Sylabs, Inc. All rights reserved.
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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/sylabs/singularity-cri/pkg/image"
	"github.com/sylabs/singularity-cri/pkg/rand"
	"github.com/sylabs/singularity-cri/pkg/singularity"
	"github.com/sylabs/singularity-cri/pkg/singularity/runtime"
	"github.com/sylabs/singularity/pkg/ociruntime"
	"github.com/sylabs/singularity/pkg/util/unix"
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
	pod      *Pod
	imgInfo  *image.Info
	baseDir  string
	trashDir string

	runtimeState runtime.State
	ociState     *ociruntime.State
	logPath      string
	execEnvs     []string

	createOnce sync.Once
	isStopped  bool
	isRemoved  bool

	isStdinClosed bool
	stdin         io.WriteCloser

	cli        *runtime.CLIClient
	syncChan   <-chan runtime.State
	syncCancel context.CancelFunc
}

// NewContainer constructs Container instance. Container is thread safe to use.
func NewContainer(config *k8s.ContainerConfig, pod *Pod, info *image.Info, trashDir string) *Container {
	contID := rand.GenerateID(ContainerIDLen)
	var execEnvs []string
	if info.OciConfig != nil {
		execEnvs = info.OciConfig.Env
	}
	// environments from config will override oci image values
	for _, kv := range config.GetEnvs() {
		execEnvs = append(execEnvs, fmt.Sprintf("%s=%s", kv.Key, kv.Value))
	}
	return &Container{
		id:              contID,
		ContainerConfig: config,
		pod:             pod,
		imgInfo:         info,
		cli:             runtime.NewCLIClient(),
		trashDir:        trashDir,
		execEnvs:        execEnvs,
	}
}

// ID returns unique container ID.
func (c *Container) ID() string {
	return c.id
}

// PodID returns ID of a pod container is executed in.
func (c *Container) PodID() string {
	return c.pod.id
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
	if c.ociState.CreatedAt == nil {
		return 0
	}
	return *c.ociState.CreatedAt
}

// StartedAt returns container start time in unix nano.
func (c *Container) StartedAt() int64 {
	if c.ociState.StartedAt == nil {
		return 0
	}
	return *c.ociState.StartedAt
}

// FinishedAt returns container finish time in unix nano.
func (c *Container) FinishedAt() int64 {
	if c.ociState.FinishedAt == nil {
		return 0
	}
	return *c.ociState.FinishedAt
}

// ExitCode returns container exit code.
func (c *Container) ExitCode() int32 {
	if c.ociState.ExitCode == nil {
		return 0
	}
	return int32(*c.ociState.ExitCode)
}

// ExitDescription returns human readable message of why container has exited.
func (c *Container) ExitDescription() string {
	return c.ociState.ExitDesc
}

// StateReason returns brief string explaining why container is in its current state.
// K8s requires us to return CamelCase here, but we will fallback to full description
// in case of unknown container state.
func (c *Container) StateReason() string {
	const (
		reasonCompleted = "Completed"
		reasonError     = "Error"
	)

	if c.runtimeState == runtime.StateRunning {
		// no need for any reason here
		return ""
	}

	if c.runtimeState == runtime.StateExited {
		if c.ExitCode() == 0 {
			return reasonCompleted
		}
		return reasonError
	}

	// fallback to the description as a last resort
	return c.ociState.ExitDesc
}

// AttachSocket returns attach socket on which runtime will serve attach request.
func (c *Container) AttachSocket() string {
	return c.ociState.AttachSocket
}

// ControlSocket returns control socket on which runtime will wait for
// control signals, e.g. resize event.
func (c *Container) ControlSocket() string {
	return c.ociState.ControlSocket
}

// LogPath returns and absolute path to container logs on the host
// filesystem or empty string if logs are not collected.
func (c *Container) LogPath() string {
	return c.logPath
}

// ImageID returns id of the container base image.
func (c *Container) ImageID() string {
	return c.imgInfo.ID
}

// Stdin returns write end of container's stdin, if any. If container
// is created with StdinOnce set to true this call will return
// nil after first attach to container finishes.
func (c *Container) Stdin() io.Writer {
	if c.isStdinClosed {
		return nil
	}
	return c.stdin
}

// CloseStdin closes write end of container's stdin.
func (c *Container) CloseStdin() error {
	if c.isStdinClosed || c.stdin == nil {
		return nil
	}
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("could not close stdin: %v", err)
	}
	c.isStdinClosed = true
	return nil
}

// Create creates container inside a pod from the image.
// All files created (bundle, sync socket, etc) are located in baseDir.
func (c *Container) Create(baseDir string) error {
	var err error
	defer func() {
		if err != nil {
			c.imgInfo.Return(c.id)
			if err := c.kill(); err != nil {
				glog.Errorf("Could not kill container after failed run: %v", err)
			}
			if err := c.cli.Delete(c.id); err != nil {
				glog.Errorf("Could not delete container: %v", err)
			}
			if err := c.collectTrash(); err != nil {
				glog.Errorf("Could not collect container trash: %v", err)
			}
			if err := c.cleanupFiles(true); err != nil {
				glog.Errorf("Could not cleanup bundle: %v", err)
			}
		}
	}()

	c.createOnce.Do(func() {
		c.baseDir = baseDir
		err = c.validateConfig()
		if err != nil {
			err = fmt.Errorf("invalid container config: %v", err)
			return
		}
		err = c.addLogDirectory()
		if err != nil {
			err = fmt.Errorf("could not create log directory: %v", err)
			return
		}
		c.imgInfo.Borrow(c.id)
		err = c.spawnOCIContainer()
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
	glog.V(3).Infof("Starting container %s", c.id)
	if err := c.cli.Start(c.id); err != nil {
		return fmt.Errorf("could not start container: %v", err)
	}
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
	err := c.UpdateState()
	if err != nil && err != runtime.ErrNotFound {
		return fmt.Errorf("could not update container state: %v", err)
	}
	if err == nil {
		if err := c.kill(); err != nil {
			return fmt.Errorf("could not kill container: %v", err)
		}
		if err := c.cli.Delete(c.id); err != nil && err != runtime.ErrNotFound {
			return fmt.Errorf("could not delete container: %v", err)
		}
	}
	if err := c.CloseStdin(); err != nil {
		glog.Errorf("Could not close container stdin: %v", err)
	}
	if err := c.collectTrash(); err != nil {
		glog.Errorf("Could not collect container trash: %v", err)
	}
	if err := c.cleanupFiles(false); err != nil {
		glog.Errorf("Container cleanup failed: %v", err)
	}
	c.imgInfo.Return(c.id)
	c.pod.removeContainer(c)
	c.isRemoved = true
	return nil
}

// ExecSync runs passed command inside a container and returns result.
func (c *Container) ExecSync(timeout time.Duration, cmd []string) (*k8s.ExecSyncResponse, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if c.imgInfo.Ref.URI() != singularity.DockerDomain || c.imgInfo.OciConfig == nil {
		cmd = append([]string{singularity.ExecScript}, cmd...)
	}
	resp, err := c.cli.ExecSync(ctx, c.id, cmd, c.execEnvs)
	if err != nil {
		return nil, fmt.Errorf("exec sync returned error: %v", err)
	}

	return &k8s.ExecSyncResponse{
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
		ExitCode: resp.ExitCode,
	}, nil
}

// Exec executes a command inside a container with attaching passed io streams to it.
func (c *Container) Exec(cmd []string, stdin io.Reader, stdout, stderr io.Writer) error {
	ctx := context.Background()

	if c.imgInfo.Ref.URI() != singularity.DockerDomain || c.imgInfo.OciConfig == nil {
		cmd = append([]string{singularity.ExecScript}, cmd...)
	}
	err := c.cli.Exec(ctx, c.id, stdin, stdout, stderr, cmd, c.execEnvs)
	if err != nil {
		return fmt.Errorf("exec returned error: %v", err)
	}

	return nil
}

// PrepareExec creates an instance of exec.Cmd that may be used
// later to run a command inside an allocated tty.
func (c *Container) PrepareExec(cmd []string) *exec.Cmd {
	ctx := context.Background()
	if c.imgInfo.Ref.URI() != singularity.DockerDomain || c.imgInfo.OciConfig == nil {
		cmd = append([]string{singularity.ExecScript}, cmd...)
	}
	return c.cli.PrepareExec(ctx, c.id, cmd, c.execEnvs)
}

// ReopenLogFile reopens container log file.
// This method is usually called when logs are rotated.
func (c *Container) ReopenLogFile() error {
	socket := c.ControlSocket()
	if socket == "" {
		return fmt.Errorf("container didn't provide control socket")

	}
	ctrlSock, err := unix.Dial(socket)
	if err != nil {
		return fmt.Errorf("could not connect to control socket: %v", err)
	}
	defer ctrlSock.Close()

	ctrl := ociruntime.Control{
		ReopenLog: true,
	}
	err = json.NewEncoder(ctrlSock).Encode(&ctrl)
	if err != nil {
		return fmt.Errorf("could not send reopen log to control socket: %v", err)
	}

	buf := make([]byte, 1)
	_, err = ctrlSock.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("could not wait reopen log: %v", err)
	}
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

	if filter.PodSandboxId != "" && filter.PodSandboxId != c.pod.id {
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
