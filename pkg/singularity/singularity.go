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

package singularity

const (
	// RuntimeName is the official name of Singularity runtime.
	RuntimeName = "singularity"

	// LibraryDomain holds the sylabs cloud library primary domain.
	// For more info refer to https://cloud.sylabs.io/library.
	LibraryDomain = "cloud.sylabs.io"

	// LocalFileDomain is a special case domain that should be used
	// for a pre-pulled SIF images.
	LocalFileDomain = "local.file"

	// DockerDomain holds docker primary domain to pull images from.
	DockerDomain = "docker.io"

	// DockerProtocol holds docker hub base URI.
	DockerProtocol = "docker"

	// KeysServer is a default singularity key management and verification server.
	KeysServer = "https://keys.sylabs.io"

	// ExecScript is a path to a shell script that should wrap any command to execute
	// inside a container based on a native SIF image.
	ExecScript = "/.singularity.d/actions/exec"

	// RunScript is a path to a shell script that should be used as a default container
	// entrypoint based on a native SIF image.
	RunScript = "/.singularity.d/actions/run"

	// EnvDockerUsername should be used to set Docker username for
	// build engine when building from a private registry.
	EnvDockerUsername = "SINGULARITY_DOCKER_USERNAME"

	// EnvDockerPassword should be used to set Docker password for
	// build engine when building from a private registry.
	EnvDockerPassword = "SINGULARITY_DOCKER_PASSWORD"
)
