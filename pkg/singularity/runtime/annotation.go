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

const (
	// AnnotationCreatedAt is used to pass creation timestamp in annotations.
	AnnotationCreatedAt = "io.sylabs.runtime.oci.created_at"

	// AnnotationStartedAt is used to pass startup timestamp in annotations.
	AnnotationStartedAt = "io.sylabs.runtime.oci.starter_at"

	// AnnotationFinishedAt is used to pass finished timestamp in annotations.
	AnnotationFinishedAt = "io.sylabs.runtime.oci.finished_at"

	// AnnotationExitCode is used to pass exit code in annotations.
	AnnotationExitCode = "io.sylabs.runtime.oci.exit-code"

	// AnnotationExitDesc is used to pass exit descrition (e.g. reson) in annotations.
	AnnotationExitDesc = "io.sylabs.runtime.oci.exit-desc"

	// AnnotationAttachSocket is used to pass socket address that runtime
	// opens for streaming requests.
	AnnotationAttachSocket = "io.sylabs.runtime.oci.attach-socket"
)
