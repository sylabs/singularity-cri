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

package image

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/sylabs/singularity-cri/pkg/singularity"
	"github.com/sylabs/singularity-cri/pkg/slice"
)

// Reference holds parsed content of image reference.
type Reference struct {
	uri string

	mu      sync.Mutex
	tags    []string
	digests []string
}

// String returns first tag or digest found with origin domain as a prefix.
func (r *Reference) String() string {
	var ref string
	if len(r.tags) > 0 {
		ref = r.tags[0]
	} else {
		ref = r.digests[0]
	}
	if r.uri == singularity.DockerDomain {
		ref = singularity.DockerDomain + "/" + ref
	}
	return ref
}

// MarshalJSON marshals Reference into a valid JSON.
func (r *Reference) MarshalJSON() ([]byte, error) {
	jsonRef := struct {
		URI     string   `json:"uri"`
		Tags    []string `json:"tags"`
		Digests []string `json:"digests"`
	}{
		URI:     r.uri,
		Tags:    r.tags,
		Digests: r.digests,
	}
	return json.Marshal(jsonRef)
}

// UnmarshalJSON unmarshals a valid Reference JSON into an object.
func (r *Reference) UnmarshalJSON(data []byte) error {
	jsonRef := struct {
		URI     string   `json:"uri"`
		Tags    []string `json:"tags"`
		Digests []string `json:"digests"`
	}{}
	err := json.Unmarshal(data, &jsonRef)
	r.uri = jsonRef.URI
	r.tags = jsonRef.Tags
	r.digests = jsonRef.Digests
	return err
}

// ParseRef constructs image reference based on imgRef.
func ParseRef(imgRef string) (*Reference, error) {
	if strings.HasPrefix(imgRef, singularity.LocalFileDomain) {
		return &Reference{
			uri:  singularity.LocalFileDomain,
			tags: []string{imgRef},
		}, nil
	}

	imgRef = NormalizedImageRef(imgRef)
	uri := singularity.DockerDomain
	if strings.HasPrefix(imgRef, singularity.LibraryDomain) {
		uri = singularity.LibraryDomain
	}

	ref := Reference{
		uri: uri,
	}

	switch uri {
	case singularity.LibraryDomain:
		if strings.Contains(imgRef, "sha256.") {
			ref.digests = []string{imgRef}
		} else {
			ref.tags = []string{imgRef}
		}
	case singularity.DockerDomain:
		if strings.IndexByte(imgRef, '@') != -1 {
			ref.digests = []string{imgRef}
		} else {
			ref.tags = []string{imgRef}
		}
	}

	return &ref, nil
}

// URI returns uri from which image was originally pulled.
func (r *Reference) URI() string {
	if r == nil {
		return ""
	}
	return r.uri
}

// Digests returns all digests referencing the image.
func (r *Reference) Digests() []string {
	digestsCopy := make([]string, len(r.digests))
	copy(digestsCopy, r.digests)
	return digestsCopy
}

// Tags returns all tags referencing the image.
func (r *Reference) Tags() []string {
	tagsCopy := make([]string, len(r.tags))
	copy(tagsCopy, r.tags)
	return tagsCopy
}

// AddDigests adds digests to image reference making sure no duplicates appear.
func (r *Reference) AddDigests(digests []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.digests = slice.MergeString(r.digests, digests...)
}

// AddTags adds tags to image reference making sure no duplicates appear.
func (r *Reference) AddTags(tags []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags = slice.MergeString(r.tags, tags...)
}

// RemoveDigest removes digest from reference.
func (r *Reference) RemoveDigest(digest string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.digests = slice.RemoveFromString(r.digests, digest)
}

// RemoveTag removes tag from reference.
func (r *Reference) RemoveTag(tag string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags = slice.RemoveFromString(r.tags, tag)
}

// NormalizedImageRef appends tag 'latest' if the passed ref
// does not have any tag or digest already. It also trims
// default docker domain prefix if present.
func NormalizedImageRef(imgRef string) string {
	if strings.HasPrefix(imgRef, singularity.LocalFileDomain) {
		return imgRef
	}

	imgRef = strings.TrimPrefix(imgRef, singularity.DockerDomain+"/")
	i := strings.LastIndexByte(imgRef, ':')
	if i == -1 {
		return imgRef + ":latest"
	}
	return imgRef
}
