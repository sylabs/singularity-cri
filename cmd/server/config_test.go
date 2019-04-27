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
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	tempConfig, err := ioutil.TempFile("", "")
	require.NoError(t, err, "could not create temp file")
	defer os.Remove(tempConfig.Name())
	defer tempConfig.Close()

	_, err = tempConfig.WriteString(`
listenSocket: /home/user/singularity.sock
storageDir: /var/lib/cri-images
streamingURL: 127.0.0.12:8080
cniBinDir: /opt/cni/bin
cniConfDir: /etc/cni/net.d
baseRunDir: /var/run/cri
`)

	require.NoError(t, err, "could not write test YAML config")
	require.NoError(t, tempConfig.Close(), "could not close test config file")

	invalidConfig, err := ioutil.TempFile("", "")
	require.NoError(t, err, "could not create invalid config file")
	defer os.Remove(invalidConfig.Name())
	defer invalidConfig.Close()
	_, err = invalidConfig.WriteString(`plain text`)
	require.NoError(t, err, "could not write invalid YAML config")
	require.NoError(t, invalidConfig.Close(), "could not close invalid config file")

	tt := []struct {
		name         string
		configPath   string
		expectConfig Config
		expectError  error
	}{
		{
			name:       "all ok",
			configPath: tempConfig.Name(),
			expectConfig: Config{
				ListenSocket: "/home/user/singularity.sock",
				StorageDir:   "/var/lib/cri-images",
				StreamingURL: "127.0.0.12:8080",
				CNIBinDir:    "/opt/cni/bin",
				CNIConfDir:   "/etc/cni/net.d",
				BaseRunDir:   "/var/run/cri",
			},
			expectError: nil,
		},
		{
			name:         "file not found",
			configPath:   "/temp/foo/bar",
			expectConfig: defaultConfig,
			expectError:  nil,
		},
		{
			name:         "invalid format",
			configPath:   invalidConfig.Name(),
			expectConfig: Config{},
			expectError:  fmt.Errorf("could not decode config: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `plain text` into main.Config"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseConfig(tc.configPath)
			require.Equal(t, tc.expectError, err)
			require.Equal(t, tc.expectConfig, actual)
		})
	}
}

func TestValidConfig(t *testing.T) {
	tt := []struct {
		name         string
		input        Config
		expectConfig Config
		expectError  error
	}{
		{
			name: "missing listen addr",
			input: Config{
				StorageDir:   "/var/lib/singularity",
				StreamingURL: "127.0.0.10:8080",
				CNIBinDir:    "/my/test/cni/bin",
				CNIConfDir:   "/etc/cni/config",
				BaseRunDir:   "/var/run/cri",
			},
			expectConfig: Config{},
			expectError:  fmt.Errorf("socket to serve cannot be empty"),
		},
		{
			name: "missing pull directory",
			input: Config{
				ListenSocket: "/var/run/sycri.sock",
				StreamingURL: "127.0.0.10:8080",
				CNIBinDir:    "/my/test/cni/bin",
				CNIConfDir:   "/etc/cni/config",
				BaseRunDir:   "/var/run/cri",
			},
			expectConfig: Config{},
			expectError:  fmt.Errorf("directory to pull images cannot be empty"),
		},
		{
			name: "missing run directory",
			input: Config{
				ListenSocket: "/var/run/sycri.sock",
				StorageDir:   "/var/lib/singularity",
				StreamingURL: "127.0.0.10:8080",
				CNIBinDir:    "/my/test/cni/bin",
				CNIConfDir:   "/etc/cni/config",
			},
			expectConfig: Config{},
			expectError:  fmt.Errorf("directory to run containers cannot be empty"),
		},
		{
			name: "minimum valid",
			input: Config{
				ListenSocket: "/var/run/sycri.sock",
				StorageDir:   "/var/lib/singularity",
				BaseRunDir:   "/var/run/cri",
			},
			expectConfig: Config{
				ListenSocket: "/var/run/sycri.sock",
				StorageDir:   "/var/lib/singularity",
				BaseRunDir:   "/var/run/cri",
			},
			expectError: nil,
		},
		{
			name: "all filled",
			input: Config{
				ListenSocket: "/var/run/sycri.sock",
				StorageDir:   "/var/lib/singularity",
				StreamingURL: "127.0.0.10:8080",
				CNIBinDir:    "/my/test/cni/bin",
				CNIConfDir:   "/etc/cni/config",
				BaseRunDir:   "/var/run/cri",
			},
			expectConfig: Config{
				ListenSocket: "/var/run/sycri.sock",
				StorageDir:   "/var/lib/singularity",
				StreamingURL: "127.0.0.10:8080",
				CNIBinDir:    "/my/test/cni/bin",
				CNIConfDir:   "/etc/cni/config",
				BaseRunDir:   "/var/run/cri",
			},
			expectError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := validConfig(tc.input)
			require.Equal(t, tc.expectError, err)
			require.Equal(t, tc.expectConfig, actual)

		})
	}
}
