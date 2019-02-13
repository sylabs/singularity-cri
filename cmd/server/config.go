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

package main

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

// Config hold all possible parameters that are used to
// tune Singularity CRI default behaviour.
type Config struct {
	// ListenSocket is a unix socket to serve CRI requests on.
	ListenSocket string `yaml:"listenSocket"`
	// StorageDir is a directory to store all pulled images in.
	StorageDir string `yaml:"storageDir"`
	// StreamingURL is an address to serve streaming requests on (exec, attach, portforward).
	StreamingURL string `yaml:"streamingURL"`
	// CNIBinDir is a directory to look for CNI plugin binaries.
	CNIBinDir string `yaml:"cniBinDir"`
	// CNIConfDir is a directory to look for CNI network configuration files.
	CNIConfDir string `yaml:"cniConfDir"`
	// BaseRunDir is a directory to store currently running pods and containers.
	BaseRunDir string `yaml:"baseRunDir"`
	// TrashDir is a directory where all container logs and configs will
	// be stored upon removal. Useful for debugging.
	TrashDir string `yaml:"trashDir"`
	// When Debug is true all CRI requests and responses will be logged. When false
	// only requests with error responses will be logged.
	Debug bool `yaml:"debug"`
}

var defaultConfig = Config{
	ListenSocket: "/var/run/singularity.sock",
	StorageDir:   "/var/lib/singularity",
	BaseRunDir:   "/var/run/singularity",
}

func parseConfig(path string) (Config, error) {
	var config Config

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		glog.Warningf("No config file found, using default")
		return defaultConfig, nil
	}
	if err != nil {
		return config, fmt.Errorf("could not open config file: %v", err)
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(&config)
	if err != nil {
		return config, fmt.Errorf("could not decode config: %v", err)
	}
	return validConfig(config)
}

func validConfig(config Config) (Config, error) {
	if config.ListenSocket == "" {
		return Config{}, fmt.Errorf("socket to serve cannot be empty")
	}
	if config.StorageDir == "" {
		return Config{}, fmt.Errorf("directory to pull images cannot be empty")
	}
	if config.BaseRunDir == "" {
		return Config{}, fmt.Errorf("directory to run containers cannot be empty")
	}
	return config, nil
}
