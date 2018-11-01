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

type CLIClient struct {
	baseCmd []string
}

func NewCLIClient() *CLIClient {
	return &CLIClient{baseCmd: []string{singularity.RuntimeName, "oci"}}
}

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

func (c *CLIClient) Run(id, bundle string) error {
	cmd := append(c.baseCmd, "run", "-b", bundle, id)
	return silentRun(cmd)
}

func (c *CLIClient) Create(id, bundle string) error {
	cmd := append(c.baseCmd, "create", "-b", bundle, id)
	return silentRun(cmd)
}

func (c *CLIClient) Start(id string) error {
	cmd := append(c.baseCmd, "start", id)
	return silentRun(cmd)
}

func (c *CLIClient) Kill(id string, force bool) error {
	sig := "SIGTERM"
	if force {
		sig = "SIGKILL"
	}
	cmd := append(c.baseCmd, "kill", id, sig)
	return silentRun(cmd)
}

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
