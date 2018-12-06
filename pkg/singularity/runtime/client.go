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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/sylabs/cri/pkg/singularity"
	"github.com/sylabs/singularity/pkg/ociruntime"
)

const (
	execScript = "/.singularity.d/actions/exec"
)

type (
	// CLIClient is a type for convenient interaction with
	// singularity OCI runtime engine via CLI.
	CLIClient struct {
		baseCmd []string
	}

	// ExecResponse holds result of command execution inside a container.
	ExecResponse struct {
		// Captured command stdout output.
		Stdout []byte
		// Captured command stderr output.
		Stderr []byte
		// Exit code the command finished with.
		ExitCode int32
	}
)

// NewCLIClient returns new CLIClient ready to use.
func NewCLIClient() *CLIClient {
	return &CLIClient{baseCmd: []string{singularity.RuntimeName, "-s", "oci"}}
}

// State returns state of a container with passed id.
func (c *CLIClient) State(id string) (*ociruntime.State, error) {
	cmd := append(c.baseCmd, "state", id)

	var cliResp bytes.Buffer
	stateCmd := exec.Command(cmd[0], cmd[1:]...)
	stateCmd.Stderr = ioutil.Discard
	stateCmd.Stdout = &cliResp

	if err := stateCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not query state: %v", err)
	}

	var state *ociruntime.State
	err := json.Unmarshal(cliResp.Bytes(), &state)
	if err != nil {
		return nil, fmt.Errorf("could not decode state: %v", err)
	}
	return state, nil
}

// Run is helper for running Create and Start is a row.
func (c *CLIClient) Run(id, bundle string, flags ...string) error {
	if err := c.Create(id, bundle, flags...); err != nil {
		return err
	}
	return c.Start(id)
}

// Create asks runtime to create a container with passed parameters.
func (c *CLIClient) Create(id, bundle string, flags ...string) error {
	cmd := append(c.baseCmd, "create")
	cmd = append(cmd, flags...)
	cmd = append(cmd, "-b", bundle, id)
	return run(cmd)
}

// Start asks runtime to start container with passed id.
func (c *CLIClient) Start(id string) error {
	cmd := append(c.baseCmd, "start", id)
	return run(cmd)
}

// ExecSync executes a command inside a container synchronously until
// context is done and returns the result.
func (c *CLIClient) ExecSync(ctx context.Context, id string, args ...string) (*ExecResponse, error) {
	cmd := append(c.baseCmd, "exec")
	cmd = append(cmd, id, execScript)
	cmd = append(cmd, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	log.Printf("executing %v", cmd)
	err := runCmd.Run()
	var exitCode int32
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		var waitStatus syscall.WaitStatus
		waitStatus, ok = exitErr.Sys().(syscall.WaitStatus)
		if ok {
			exitCode = int32(waitStatus.ExitStatus())
		}
	}
	if !ok && err != nil {
		return nil, fmt.Errorf("could not execute: %v", err)
	}
	return &ExecResponse{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// Exec executes passed command inside a container setting io streams to passed ones.
func (c *CLIClient) Exec(ctx context.Context, id string,
	stdin io.Reader, stdout, stderr io.WriteCloser,
	args ...string) error {

	runCmd := c.PrepareExec(ctx, id, args...)
	runCmd.Stdout = stdout
	runCmd.Stderr = stderr
	runCmd.Stdin = stdin

	err := runCmd.Run()
	_, ok := err.(*exec.ExitError)
	if !ok && err != nil {
		return fmt.Errorf("could not execute: %v", err)
	}
	return nil
}

// PrepareExec simply prepares command to call to execute inside a
// given container. It makes sure singularity exec script is called.
func (c *CLIClient) PrepareExec(ctx context.Context, id string, args ...string) *exec.Cmd {
	cmd := append(c.baseCmd, "exec")
	cmd = append(cmd, id, execScript)
	cmd = append(cmd, args...)

	log.Printf("will execute %v", cmd)
	return exec.CommandContext(ctx, cmd[0], cmd[1:]...)
}

// Kill asks runtime to send SIGINT to container with passed id.
// If force is true that SIGKILL is sent instead.
func (c *CLIClient) Kill(id string, force bool) error {
	sig := "SIGINT"
	if force {
		sig = "SIGKILL"
	}
	cmd := append(c.baseCmd, "kill", "-s", sig, id)
	return run(cmd)
}

// Delete asks runtime to delete container with passed id.
func (c *CLIClient) Delete(id string) error {
	cmd := append(c.baseCmd, "delete", id)
	return run(cmd)
}

// Attach asks runtime attach to container standard streams.
func (c *CLIClient) Attach(id string) error {
	cmd := append(c.baseCmd, "attach", id)
	return run(cmd)
}

func run(cmd []string) error {
	runCmd := exec.Command(cmd[0], cmd[1:]...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = os.Stdout

	log.Printf("executing %v", cmd)
	err := runCmd.Run()
	if err != nil {
		return fmt.Errorf("could not execute: %v", err)
	}
	return nil
}
