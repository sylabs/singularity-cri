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

package image

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sylabs/cri/pkg/singularity"
)

// Reference holds parsed content of image reference.
type Reference struct {
	uri string

	mu      sync.Mutex
	tags    []string
	digests []string
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
	uri := singularity.DockerDomain
	image := imgRef
	indx := strings.IndexByte(imgRef, '/')
	if indx != -1 {
		switch image[:indx] {
		case singularity.LibraryDomain:
			uri = singularity.LibraryDomain
			image = image[indx+1:]
		case singularity.DockerDomain:
			image = image[indx+1:]
		}
	}

	ref := Reference{
		uri: uri,
	}

	switch uri {
	case singularity.LibraryDomain:
		if strings.Contains(image, "sha256.") {
			ref.digests = append(ref.digests, imgRef)
		} else {
			ref.tags = append(ref.tags, NormalizedImageRef(imgRef))
		}
	case singularity.DockerDomain:
		if strings.IndexByte(image, '@') != -1 {
			ref.digests = append(ref.digests, image)
		} else {
			ref.tags = append(ref.tags, NormalizedImageRef(image))
		}
	default:
		return nil, fmt.Errorf("unknown image registry: %s", uri)
	}

	return &ref, nil
}

// URI returns uri from which image was originally pulled
func (r *Reference) URI() string {
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
	r.digests = mergeStrSlice(r.digests, digests)
}

// AddTags adds tags to image reference making sure no duplicates appear.
func (r *Reference) AddTags(tags []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags = mergeStrSlice(r.tags, tags)
}

// RemoveDigest removes digest from reference.
func (r *Reference) RemoveDigest(digest string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.digests = removeFromSlice(r.digests, digest)
}

// RemoveTag removes tag from reference.
func (r *Reference) RemoveTag(tag string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags = removeFromSlice(r.tags, tag)
}

// NormalizedImageRef appends tag 'latest' if the passed ref
// does not have any tag or digest already.
func NormalizedImageRef(imgRef string) string {
	i := strings.LastIndexByte(imgRef, ':')
	if i == -1 {
		return imgRef + ":latest"
	}
	return imgRef
}

func mergeStrSlice(t1, t2 []string) []string {
	unique := make(map[string]struct{})
	for _, tag := range append(t1, t2...) {
		unique[tag] = struct{}{}
	}
	merged := make([]string, 0, len(unique))
	for str := range unique {
		merged = append(merged, str)
	}
	return merged
}

// removeFromSlice returns passed slice without first occurrence of element v.
// It does not make a copy of a passed slice.
func removeFromSlice(a []string, v string) []string {
	for i, str := range a {
		if str == v {
			return append(a[:i], a[i+1:]...)
		}
	}
	return a
}
