// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package singularity

const (
	// RuntimeName is the official name of Singularity runtime.
	RuntimeName = "singularity"

	// LibraryURL is a default singularity library server address.
	LibraryURL = "https://library.sylabs.io"

	// LibraryProtocol holds the sylabs cloud library base URI.
	// For more info refer to https://cloud.sylabs.io/library.
	LibraryProtocol = "library"

	// ShubProtocol holds singularity hub base URI.
	// For more info refer to https://singularity-hub.org.
	ShubProtocol = "shub"

	// DockerProtocol holds docker hub base URI.
	DockerProtocol = "docker"
)
