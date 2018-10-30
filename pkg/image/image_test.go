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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sylabs/cri/pkg/singularity"
	"github.com/sylabs/singularity/src/pkg/util/user-agent"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func TestPullImage(t *testing.T) {
	useragent.InitValue("singularity", "3.0.0")

	tt := []struct {
		name        string
		ref         *Reference
		expectImage *Info
		expectError error
	}{
		{
			name: "docker image",
			ref: &Reference{
				uri:     "docker",
				tags:    []string{"gcr.io/cri-tools/test-image-latest"},
				digests: nil,
			},
			expectImage: &Info{
				id:     "",
				sha256: "",
				size:   741376,
				path:   "",
				ref: &Reference{
					uri:     "docker",
					tags:    []string{"gcr.io/cri-tools/test-image-latest"},
					digests: nil,
				},
			},
			expectError: nil,
		},
		{
			name: "library image",
			ref: &Reference{
				uri:     "library",
				tags:    nil,
				digests: []string{"library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8"},
			},
			expectImage: &Info{
				id:     "9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
				sha256: "9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8",
				size:   5517312,
				path:   filepath.Join(os.TempDir(), "9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8"),
				ref: &Reference{
					uri:     "library",
					tags:    nil,
					digests: []string{"library://sashayakovtseva/test/image-server:sha256.9327532a05078d7efd5a0ef9ace1ee5cd278653d8df53590e2fb7a4a34cb0bb8"},
				},
			},
			expectError: nil,
		},
		{
			name: "shub image",
			ref: &Reference{
				uri:     "shub",
				tags:    []string{"shub://vsoch/hello-world"},
				digests: nil,
			},
			expectImage: &Info{
				id:     "4d398430ceded6a261a2304df3e75efe558892ba94eec25d2392991fe3a13dce",
				sha256: "4d398430ceded6a261a2304df3e75efe558892ba94eec25d2392991fe3a13dce",
				size:   65347615,
				path:   filepath.Join(os.TempDir(), "4d398430ceded6a261a2304df3e75efe558892ba94eec25d2392991fe3a13dce"),
				ref: &Reference{
					uri:     "shub",
					tags:    []string{"shub://vsoch/hello-world"},
					digests: nil,
				},
			},
			expectError: nil,
		},
		{
			name: "unknown image",
			ref: &Reference{
				uri:     "rkt",
				tags:    []string{"rkt://vsoch/hello-world"},
				digests: nil,
			},
			expectImage: nil,
			expectError: fmt.Errorf("could not pull image: unknown image registry: rkt"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			image, err := Pull(os.TempDir(), tc.ref)
			require.Equal(t, tc.expectError, err, "could not pull image")
			if image != nil {
				require.NoError(t, image.Remove(), "could not remove image")
			}
			if tc.ref.uri == singularity.DockerProtocol {
				image.id = ""
				image.sha256 = ""
				image.path = ""
			}
			require.Equal(t, tc.expectImage, image, "image mismatch")
		})
	}
}

func TestMatches(t *testing.T) {
	tt := []struct {
		name   string
		img    *Info
		filter *k8s.ImageFilter
		expect bool
	}{
		{
			name: "no filter",
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
					digests: []string{},
				},
			},
			filter: &k8s.ImageFilter{},
			expect: true,
		},
		{
			name: "id match",
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
					digests: []string{},
				},
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
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{"gcr.io/cri-tools/test-image-tags:1", "gcr.io/cri-tools/test-image-tags:2"},
					digests: []string{},
				},
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
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{},
					digests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
				},
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
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{},
					digests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
				},
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
			img: &Info{
				id: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				ref: &Reference{
					tags:    []string{},
					digests: []string{"gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"},
				},
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
			require.Equal(t, tc.expect, tc.img.Matches(tc.filter))
		})
	}
}
