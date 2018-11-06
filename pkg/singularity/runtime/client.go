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
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sylabs/cri/pkg/singularity"
)

// CLIClient is a type for convenient interaction with
// singularity OCI runtime engine via CLI.
type CLIClient struct {
	baseCmd []string
}

// NewCLIClient returns new CLIClient ready to use.
func NewCLIClient() *CLIClient {
	return &CLIClient{baseCmd: []string{singularity.RuntimeName, "oci"}}
}

// State returns state of a container with passed id.
func (c *CLIClient) State(id string) (*specs.State, error) {
	cmd := append(c.baseCmd, "state", id)

	var cliResp bytes.Buffer
	stateCmd := exec.Command(cmd[0], cmd[1:]...)
	stateCmd.Stderr = ioutil.Discard
	stateCmd.Stdout = &cliResp

	if err := stateCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not query state: %v", err)
	}

	var state *specs.State
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
	return silentRun(cmd)
}

// Start asks runtime to start container with passed id.
func (c *CLIClient) Start(id string) error {
	cmd := append(c.baseCmd, "start", id)
	return silentRun(cmd)
}

// Kill asks runtime to send SIGTERM to container with passed id.
// If force is true that SIGKILL is sent instead.
func (c *CLIClient) Kill(id string, force bool) error {
	sig := "SIGTERM"
	if force {
		sig = "SIGKILL"
	}
	cmd := append(c.baseCmd, "kill", "-s", sig, id)
	return silentRun(cmd)
}

// Delete asks runtime to delete container with passed id.
func (c *CLIClient) Delete(id string) error {
	cmd := append(c.baseCmd, "delete", id)
	return silentRun(cmd)
}

func silentRun(cmd []string) error {
	runCmd := exec.Command(cmd[0], cmd[1:]...)
	runCmd.Stderr = os.Stderr
	runCmd.Stdout = os.Stdout

	log.Printf("executing %v", cmd)
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("could not execute %v: %v", cmd, err)
	}
	return nil
}
