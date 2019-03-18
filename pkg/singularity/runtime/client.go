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

package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"github.com/sylabs/singularity-cri/pkg/singularity"
)

type (
	// CLIClient is a type for convenient interaction with
	// singularity OCI runtime engine via CLI.
	CLIClient struct {
		ociBaseCmd []string
	}

	// BuildConfig is Singularity's build configuration.
	BuildConfig struct {
		SingularityConfdir string
	}
)

// NewCLIClient returns new CLIClient ready to use.
func NewCLIClient() *CLIClient {
	return &CLIClient{ociBaseCmd: []string{singularity.RuntimeName, "-q", "oci"}}
}

// BuildConfig returns configuration which was used to build
// current Singularity installation.
func (c *CLIClient) BuildConfig() (*BuildConfig, error) {
	cmd := exec.Command(singularity.RuntimeName, "buildcfg")
	confBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not run buildcfg command: %v", err)
	}
	conf := parseBuildConfig(confBytes)
	if conf.SingularityConfdir == "" {
		return nil, fmt.Errorf("invalid build configuration")
	}
	return &conf, nil
}

func run(cmd []string) error {
	runCmd := exec.Command(cmd[0], cmd[1:]...)
	runCmd.Stderr = os.Stderr

	glog.V(4).Infof("Executing %v", cmd)
	err := runCmd.Run()
	if err != nil {
		return fmt.Errorf("could not execute: %v", err)
	}
	return nil
}

func parseBuildConfig(data []byte) BuildConfig {
	const singularityConfdir = "SINGULARITY_CONFDIR"

	var cfg BuildConfig
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		if parts[0] == singularityConfdir {
			cfg.SingularityConfdir = parts[1]
			break
		}
	}
	return cfg
}
