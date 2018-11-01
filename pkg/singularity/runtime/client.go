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
func (c *CLIClient) Run(id, bundle string) error {
	cmd := append(c.baseCmd, "run", "-b", bundle, id)
	return silentRun(cmd)
}

// Create asks runtime to create a container with passed parameters.
func (c *CLIClient) Create(id, bundle string) error {
	cmd := append(c.baseCmd, "create", "-b", bundle, id)
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
	stateCmd := exec.Command(cmd[0], cmd[1:]...)
	stateCmd.Stderr = os.Stderr
	stateCmd.Stdout = os.Stdout

	log.Printf("executing %v", cmd)
	if err := stateCmd.Run(); err != nil {
		return fmt.Errorf("could not execute %v: %v", cmd, err)
	}
	return nil
}
