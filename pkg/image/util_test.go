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
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func TestParseImageRef(t *testing.T) {
	tt := []struct {
		name        string
		ref         string
		expect      imageInfo
		expectError error
	}{
		{
			name:        "invalid uri",
			ref:         "rkt://library/ubuntu:16.4",
			expect:      imageInfo{},
			expectError: fmt.Errorf("unknown image registry: rkt"),
		},
		{
			name: "library with tag",
			ref:  "library://sashayakovtseva/test/image-server:1",
			expect: imageInfo{
				Origin:  "library",
				Tags:    []string{"library://sashayakovtseva/test/image-server:1"},
				Digests: nil,
			},
			expectError: nil,
		},
		{
			name: "library without tag",
			ref:  "library://sashayakovtseva/test/image-server",
			expect: imageInfo{
				Origin:  "library",
				Tags:    []string{"library://sashayakovtseva/test/image-server:latest"},
				Digests: nil,
			},
			expectError: nil,
		},
		{
			name: "library with digest",
			ref:  "library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
			expect: imageInfo{
				Origin:  "library",
				Tags:    nil,
				Digests: []string{"library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8"},
			},
			expectError: nil,
		},
		{
			name: "shub without tag",
			ref:  "shub://vsoch/singularity-hello-world",
			expect: imageInfo{
				Origin:  "shub",
				Tags:    []string{"shub://vsoch/singularity-hello-world:latest"},
				Digests: nil,
			},
			expectError: nil,
		},
		{
			name: "docker without tag",
			ref:  "gcr.io/cri-tools/test-image-tags",
			expect: imageInfo{
				Origin:  "docker",
				Tags:    []string{"gcr.io/cri-tools/test-image-tags:latest"},
				Digests: nil,
			},
			expectError: nil,
		},
		{
			name: "docker with tag",
			ref:  "docker://gcr.io/cri-tools/test-image-tags:1",
			expect: imageInfo{
				Origin:  "docker",
				Tags:    []string{"gcr.io/cri-tools/test-image-tags:1"},
				Digests: nil,
			},
			expectError: nil,
		},
		{
			name: "docker with digest",
			ref:  "docker://gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343",
			expect: imageInfo{
				Origin:  "docker",
				Tags:    nil,
				Digests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
			},
			expectError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseImageRef(tc.ref)
			require.Equal(t, tc.expectError, err)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestPullImage(t *testing.T) {
	tt := []struct {
		name         string
		image        imageInfo
		expectDigest string
		expectError  error
	}{
		{
			name: "docker image",
			image: imageInfo{
				Origin:  "docker",
				Tags:    []string{"gcr.io/cri-tools/test-image-latest"},
				Digests: nil,
			},
			expectDigest: "",
			expectError:  nil,
		},
		{
			name: "library image",
			image: imageInfo{
				Origin:  "library",
				Tags:    nil,
				Digests: []string{"library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8"},
			},
			expectDigest: "9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
			expectError:  nil,
		},
		{
			name: "shub image",
			image: imageInfo{
				Origin:  "shub",
				Tags:    []string{"shub://vsoch/hello-world"},
				Digests: nil,
			},
			expectDigest: "4d398430ceded6a261a2304df3e75efe558892ba94eec25d2392991fe3a13dce",
			expectError:  nil,
		},
		{
			name: "unknown image",
			image: imageInfo{
				Origin:  "rkt",
				Tags:    []string{"rkt://vsoch/hello-world"},
				Digests: nil,
			},
			expectError: fmt.Errorf("unknown image registry: rkt"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			temp, err := ioutil.TempFile("", "")
			require.NoError(t, err, "could not create temp file")
			defer os.Remove(temp.Name())
			defer temp.Close()

			err = pullImage(nil, temp.Name(), tc.image)
			require.Equal(t, tc.expectError, err, "could not pull image")

			if tc.expectDigest != "" {
				h := sha256.New()
				_, err = io.Copy(h, temp)
				require.NoError(t, err, "could not find file checksum")
				sum := fmt.Sprintf("%x", h.Sum(nil))
				require.Equal(t, tc.expectDigest, sum)
			}
		})
	}
}

func TestNormalizedImageRef(t *testing.T) {
	tt := []struct {
		name   string
		ref    string
		expect string
	}{
		{
			name:   "docker image with tag",
			ref:    "gcr.io/cri-tools/test-image-tags:1",
			expect: "gcr.io/cri-tools/test-image-tags:1",
		},
		{
			name:   "docker image without tag",
			ref:    "gcr.io/cri-tools/test-image-tags",
			expect: "gcr.io/cri-tools/test-image-tags:latest",
		},
		{
			name:   "docker image with digest",
			ref:    "gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343",
			expect: "gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343",
		},
		{
			name:   "docker image with tag",
			ref:    "library://sashayakovtseva/test/image-server:latest",
			expect: "library://sashayakovtseva/test/image-server:latest",
		},
		{
			name:   "library image without tag",
			ref:    "library://sashayakovtseva/test/image-server",
			expect: "library://sashayakovtseva/test/image-server:latest",
		},
		{
			name:   "library image with digest",
			ref:    "library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
			expect: "library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := normalizedImageRef(tc.ref)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestMergeStrSlice(t *testing.T) {
	tt := []struct {
		name   string
		s1     []string
		s2     []string
		expect []string
	}{
		{
			name:   "no intersection",
			s1:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			s2:     []string{"gcr.io/cri-tools/test-image-tags:3"},
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
		{
			name:   "intersection",
			s1:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			s2:     []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:3"},
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := mergeStrSlice(tc.s1, tc.s2)
			sort.Strings(actual)
			sort.Strings(tc.expect)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestRemoveFromSlice(t *testing.T) {
	tt := []struct {
		name   string
		s      []string
		v      string
		expect []string
	}{
		{
			name:   "not found",
			s:      []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
			v:      "gcr.io/cri-tools/test-image-tags:3",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
		},
		{
			name:   "single occurrence",
			s:      []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
			v:      "gcr.io/cri-tools/test-image-tags:2",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:3"},
		},
		{
			name:   "multiple occurrence",
			s:      []string{"gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
			v:      "gcr.io/cri-tools/test-image-tags:2",
			expect: []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2", "gcr.io/cri-tools/test-image-tags:3"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := removeFromSlice(tc.s, tc.v)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestMatches(t *testing.T) {
	tt := []struct {
		name   string
		img    *k8s.Image
		filter *k8s.ImageFilter
		expect bool
	}{
		{
			name: "no filter",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
				RepoDigests: []string{},
			},
			filter: &k8s.ImageFilter{},
			expect: true,
		},
		{
			name: "id match",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
				RepoDigests: []string{},
			},
			filter: &k8s.ImageFilter{
				Image: &k8s.ImageSpec{
					Image: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				},
			},
			expect: true,
		},
		{
			name: "tag match",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
				RepoDigests: []string{},
			},
			filter: &k8s.ImageFilter{
				Image: &k8s.ImageSpec{
					Image: "gcr.io/cri-tools/test-image-tags",
				},
			},
			expect: true,
		},
		{
			name: "digest match",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{},
				RepoDigests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
			},
			filter: &k8s.ImageFilter{
				Image: &k8s.ImageSpec{
					Image: "gcr.io/cri-tools/test-image-digest",
				},
			},
			expect: true,
		},
		{
			name: "empty filter",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{},
				RepoDigests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
			},
			filter: &k8s.ImageFilter{
				Image: &k8s.ImageSpec{
					Image: "",
				},
			},
			expect: true,
		},
		{
			name: "no match",
			img: &k8s.Image{
				Id:          "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				RepoTags:    []string{},
				RepoDigests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
			},
			filter: &k8s.ImageFilter{
				Image: &k8s.ImageSpec{
					Image: "gcr.io/cri-tools/test-image-tags",
				},
			},
			expect: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := matches(tc.img, tc.filter)
			require.Equal(t, tc.expect, actual)
		})
	}
}
