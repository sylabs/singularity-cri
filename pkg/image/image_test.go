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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/sylabs/singularity-cri/pkg/singularity"
	useragent "github.com/sylabs/singularity/pkg/util/user-agent"
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
				uri:     singularity.DockerDomain,
				tags:    []string{"gcr.io/cri-tools/test-image-latest"},
				digests: nil,
			},
			expectImage: &Info{
				id:     "",
				sha256: "",
				size:   745472,
				path:   "",
				ref: &Reference{
					uri:     singularity.DockerDomain,
					tags:    []string{"gcr.io/cri-tools/test-image-latest"},
					digests: nil,
				},
				ociConfig: &specs.ImageConfig{
					Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
					Cmd: []string{"sh"},
				},
			},
			expectError: nil,
		},
		{
			name: "library image",
			ref: &Reference{
				uri:     singularity.LibraryDomain,
				tags:    nil,
				digests: []string{"cloud.sylabs.io/sashayakovtseva/test/image-server:sha256.d50278eebfe4ca5655cc28503983f7c947914a34fbbb805481657d39e98f33f0"},
			},
			expectImage: &Info{
				id:     "d50278eebfe4ca5655cc28503983f7c947914a34fbbb805481657d39e98f33f0",
				sha256: "d50278eebfe4ca5655cc28503983f7c947914a34fbbb805481657d39e98f33f0",
				size:   5521408,
				path:   filepath.Join(os.TempDir(), "d50278eebfe4ca5655cc28503983f7c947914a34fbbb805481657d39e98f33f0"),
				ref: &Reference{
					uri:     singularity.LibraryDomain,
					tags:    nil,
					digests: []string{"cloud.sylabs.io/sashayakovtseva/test/image-server:sha256.d50278eebfe4ca5655cc28503983f7c947914a34fbbb805481657d39e98f33f0"},
				},
			},
			expectError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			image, err := Pull(os.TempDir(), tc.ref)
			require.Equal(t, tc.expectError, err, "could not pull image")
			if image != nil {
				require.NoError(t, image.Remove(), "could not remove image")
			}
			if tc.ref.uri == singularity.DockerDomain {
				image.id = ""
				image.sha256 = ""
				image.path = ""
			}
			require.Equal(t, tc.expectImage, image, "image mismatch")
		})
	}
}

func TestInfo_Remove(t *testing.T) {
	useragent.InitValue("singularity", "3.0.0")

	tt := []struct {
		name         string
		borrow       []string
		ret          []string
		expectUsedBy []string
		expectError  error
	}{
		{
			name:        "not used",
			borrow:      nil,
			ret:         nil,
			expectError: nil,
		},
		{
			name:         "used and returned",
			borrow:       []string{"first_container"},
			ret:          []string{"first_container"},
			expectUsedBy: nil,
			expectError:  nil,
		},
		{
			name:         "used and not returned",
			borrow:       []string{"first_container"},
			ret:          nil,
			expectUsedBy: []string{"first_container"},
			expectError:  ErrIsUsed,
		},
		{
			name:         "multiple return",
			borrow:       []string{"first_container", "second_container"},
			ret:          []string{"first_container", "second_container"},
			expectUsedBy: nil,
			expectError:  nil,
		},
		{
			name:         "multiple without return",
			borrow:       []string{"first_container", "second_container"},
			ret:          []string{"second_container"},
			expectUsedBy: []string{"first_container"},
			expectError:  ErrIsUsed,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ioutil.TempFile("", "")
			require.NoError(t, err, "could not create temp image file")

			t.Logf("temp file %s", f.Name())
			defer os.Remove(f.Name())
			defer f.Close()

			image := &Info{
				path: f.Name(),
			}
			for _, b := range tc.borrow {
				image.Borrow(b)
			}
			for _, r := range tc.ret {
				image.Return(r)
			}
			actual := image.UsedBy()
			require.ElementsMatch(t, tc.expectUsedBy, actual)
			err = image.Remove()
			require.Equal(t, tc.expectError, err)
		})
	}
}

func TestInfo_Matches(t *testing.T) {
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

func TestInfo_UnmarshalJSON(t *testing.T) {
	input := `
{"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
"sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
"size":741376,
"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
"ref":{"uri":"docker.io","tags":["busybox:1.28"],"digests":null}}`

	info := new(Info)
	err := info.UnmarshalJSON([]byte(input))
	require.NoError(t, err, "could not unmarshal image")
	require.Equal(t, &Info{
		id:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		size:   741376,
		path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		ref: &Reference{
			uri:  singularity.DockerDomain,
			tags: []string{"busybox:1.28"},
		},
	}, info)
}

func TestInfo_MarshalJSON(t *testing.T) {
	expect := []byte(`{"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0","sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0","size":741376,"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0","ref":{"uri":"docker.io","tags":["busybox:1.28"],"digests":null}}`)

	info := &Info{
		id:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		size:   741376,
		path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
		ref: &Reference{
			uri:  singularity.DockerDomain,
			tags: []string{"busybox:1.28"},
		},
	}

	res, err := info.MarshalJSON()
	require.NoError(t, err, "could not marshal image")
	require.Equal(t, expect, res)
}
