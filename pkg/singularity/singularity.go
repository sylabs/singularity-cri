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

package singularity

const (
	// RuntimeName is the official name of Singularity runtime.
	RuntimeName = "singularity"

	// StarterName is the name of executable that is responsible for pod and container start.
	StarterName = "starter"

	// LibraryURL is a default singularity library server address.
	LibraryURL = "https://library.sylabs.io"

	// LibraryProtocol holds the sylabs cloud library base URI.
	// For more info refer to https://cloud.sylabs.io/library.
	LibraryProtocol = "library"

	LibraryDomain = "cloud.sylabs.io"

	DockerDomain = "docker.io"

	ShubDomain = "singularity-hub.org"

	// ShubProtocol holds singularity hub base URI.
	// For more info refer to https://singularity-hub.org.
	ShubProtocol = "shub"

	// DockerProtocol holds docker hub base URI.
	DockerProtocol = "docker"

	// KeysServer is a default singularity key management and verification server.
	KeysServer = "https://keys.sylabs.io"
)
