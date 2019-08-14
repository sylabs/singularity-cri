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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/sylabs/singularity-cri/pkg/singularity"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func TestPullImage(t *testing.T) {
	privateServer := os.Getenv("PRIVATE_SERVER")
	privateUsername := os.Getenv("PRIVATE_USERNAME")
	privatePassword := os.Getenv("PRIVATE_PASSWORD")

	tt := []struct {
		name        string
		skip        bool
		ref         *Reference
		auth        *k8s.AuthConfig
		expectImage *Info
		expectError string
	}{
		{
			name: "unknown registry",
			ref: &Reference{
				uri:  "foo.io",
				tags: []string{"foo.io/cri-tools/test-image-latest"},
			},
			expectImage: nil,
			expectError: "could not pull image: unknown image registry: foo.io",
		},
		{
			name: "docker image",
			ref: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"busybox:1.31"},
			},
			expectImage: &Info{
				Size: 782336,
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"busybox:1.31"},
				},
				OciConfig: &specs.ImageConfig{
					Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
					Cmd: []string{"sh"},
				},
			},
		},
		{
			name: "custom docker server address",
			ref: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"cri-tools/test-image-latest"},
			},
			auth: &k8s.AuthConfig{
				ServerAddress: "gcr.io",
			},
			expectImage: &Info{
				Size: 782336,
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"cri-tools/test-image-latest"},
				},
				OciConfig: &specs.ImageConfig{
					Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
					Cmd: []string{"sh"},
				},
			},
		},
		{
			name: "library by digest",
			ref: &Reference{
				uri: singularity.LibraryDomain,
				digests: []string{
					"cloud.sylabs.io/sylabs/tests/busybox:sha256.8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				},
			},
			expectImage: &Info{
				ID:     "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Sha256: "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Size:   672699,
				Path:   filepath.Join(os.TempDir(), "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba"),
				Ref: &Reference{
					uri: singularity.LibraryDomain,
					digests: []string{
						"cloud.sylabs.io/sylabs/tests/busybox:sha256.8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
					},
				},
			},
		},
		{
			name: "library by tag",
			ref: &Reference{
				uri: singularity.LibraryDomain,
				tags: []string{
					"cloud.sylabs.io/sylabs/tests/busybox:1.0.0",
				},
			},
			expectImage: &Info{
				ID:     "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Sha256: "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Size:   672699,
				Path:   filepath.Join(os.TempDir(), "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba"),
				Ref: &Reference{
					uri: singularity.LibraryDomain,
					tags: []string{
						"cloud.sylabs.io/sylabs/tests/busybox:1.0.0",
					},
				},
			},
		},
		{
			name: "private docker image without creds",
			ref: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"sylabs/test:latest"},
			},
			auth: &k8s.AuthConfig{
				ServerAddress: privateServer,
			},
			skip:        privateServer == "",
			expectError: "unauthorized: authentication required",
		},
		{
			name: "private docker image",
			ref: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"sylabs/test:latest"},
			},
			auth: &k8s.AuthConfig{
				ServerAddress: privateServer,
				Username:      privateUsername,
				Password:      privatePassword,
			},

			skip: privateServer == "" && privatePassword == "",
			expectImage: &Info{
				Size: 2723840,
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"sylabs/test:latest"},
				},
				OciConfig: &specs.ImageConfig{
					Env: []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
					Cmd: []string{"/bin/sh"},
				},
			},
		},
		{
			name: "local SIF no found",
			ref: &Reference{
				uri:  singularity.LocalFileDomain,
				tags: []string{"local.file/tmp/not-found.sif"},
			},
			expectError: "no such file or directory",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip()
			}

			image, err := Pull(context.Background(), os.TempDir(), tc.ref, tc.auth)
			if tc.expectError == "" {
				require.NoError(t, err, "unexpected error")
			} else {
				require.Error(t, err, "expected error, but got nil")
				require.Contains(t, err.Error(), tc.expectError, "unexpected pull error")
			}
			if image != nil {
				require.NoError(t, image.Remove(), "could not remove image")
			}
			if image != nil && tc.ref.URI() == singularity.DockerDomain {
				image.ID = ""
				image.Sha256 = ""
				image.Path = ""
			}
			require.Equal(t, tc.expectImage, image, "image mismatch")
		})
	}
}

func TestLibraryInfo(t *testing.T) {
	tt := []struct {
		name        string
		ref         *Reference
		expectImage *Info
		expectError error
	}{
		{
			name: "unknown registry",
			ref: &Reference{
				uri:  "foo.io",
				tags: []string{"foo.io/cri-tools/test-image-latest"},
			},
			expectError: ErrNotLibrary,
		},
		{
			name: "docker image",
			ref: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"gcr.io/cri-tools/test-image-latest"},
			},
			expectError: ErrNotLibrary,
		},
		{
			name: "library by digest",
			ref: &Reference{
				uri: singularity.LibraryDomain,
				digests: []string{
					"cloud.sylabs.io/sylabs/tests/busybox:sha256.8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				},
			},
			expectImage: &Info{
				ID:     "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Sha256: "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Size:   672699,
				Ref: &Reference{
					uri: singularity.LibraryDomain,
					digests: []string{
						"cloud.sylabs.io/sylabs/tests/busybox:sha256.8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
					},
				},
			},
		},
		{
			name: "library by tag",
			ref: &Reference{
				uri: singularity.LibraryDomain,
				tags: []string{
					"cloud.sylabs.io/sylabs/tests/busybox:1.0.0",
				},
			},
			expectImage: &Info{
				ID:     "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Sha256: "8b5478b0f2962eba3982be245986eb0ea54f5164d90a65c078af5b83147009ba",
				Size:   672699,
				Ref: &Reference{
					uri: singularity.LibraryDomain,
					tags: []string{
						"cloud.sylabs.io/sylabs/tests/busybox:1.0.0",
					},
				},
			},
		},
		{
			name: "library not found",
			ref: &Reference{
				uri:     singularity.LibraryDomain,
				digests: []string{"cloud.sylabs.io/sylabs/tests/not-found"},
			},
			expectError: ErrNotFound,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			image, err := LibraryInfo(context.Background(), tc.ref, nil)
			require.Equal(t, tc.expectError, err, "could not get library image info")
			require.Equal(t, tc.expectImage, image, "image mismatch")
		})
	}
}

func TestInfo_Verify(t *testing.T) {
	tt := []struct {
		name        string
		imgRef      *Reference
		image       *Info
		expectError string
	}{
		{
			name: "docker image",
			imgRef: &Reference{
				uri:  singularity.DockerDomain,
				tags: []string{"gcr.io/cri-tools/test-image-latest"},
			},
			expectError: "",
		},
		{
			name: "signed SIF",
			imgRef: &Reference{
				uri:  singularity.LibraryDomain,
				tags: []string{"sylabs/tests/verify_success:1.0.1"},
			},
			expectError: "",
		},
		{
			name: "non-signed SIF",
			imgRef: &Reference{
				uri:  singularity.LibraryDomain,
				tags: []string{"sylabs/tests/unsigned:1.0.0"},
			},
			expectError: "",
		},
		{
			name: "broken signature SIF",
			imgRef: &Reference{
				uri:  singularity.LibraryDomain,
				tags: []string{"sylabs/tests/verify_corrupted:1.0.1"},
			},
			expectError: "verification failed",
		},
		{
			name: "broken image info",
			image: &Info{
				Path: "/foo/bar",
				Ref: &Reference{
					uri: singularity.LibraryDomain,
				},
			},
			expectError: "open /foo/bar: no such file or directory",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			img := tc.image
			if img == nil {
				img, err = Pull(context.Background(), os.TempDir(), tc.imgRef, nil)
				require.NoError(t, err, "could not pull SIF")
				defer func() {
					require.NoError(t, img.Remove(), "could not remove SIF")
				}()
			}

			err = img.Verify()
			if tc.expectError == "" {
				require.NoError(t, err, "unexpected error")
			} else {
				require.Error(t, err, "expected error, but got nil")
				require.Contains(t, err.Error(), tc.expectError, "unexpected verify error")
			}
		})
	}
}

func TestInfo_BorrowReturn(t *testing.T) {
	tt := []struct {
		name         string
		borrow       []string
		ret          []string
		expectUsedBy []string
	}{
		{
			name: "not used",
		},
		{
			name:   "used and returned",
			borrow: []string{"first_container"},
			ret:    []string{"first_container"},
		},
		{
			name:         "used and not returned",
			borrow:       []string{"first_container"},
			expectUsedBy: []string{"first_container"},
		},
		{
			name:   "multiple return",
			borrow: []string{"first_container", "second_container"},
			ret:    []string{"first_container", "second_container"},
		},
		{
			name:         "multiple without return",
			borrow:       []string{"first_container", "second_container"},
			ret:          []string{"second_container"},
			expectUsedBy: []string{"first_container"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var image Info
			for _, b := range tc.borrow {
				image.Borrow(b)
			}
			for _, r := range tc.ret {
				image.Return(r)
			}
			actual := image.UsedBy()
			require.ElementsMatch(t, tc.expectUsedBy, actual)
		})
	}
}

func TestInfo_Remove(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err, "could not create temp image file")
	require.NoError(t, f.Close())

	defer os.Remove(f.Name())

	tt := []struct {
		name        string
		image       *Info
		expectError error
	}{
		{
			name: "non existent file",
			image: &Info{
				Path: "/foo/bar",
			},
			expectError: fmt.Errorf("could not remove image: remove /foo/bar: no such file or directory"),
		},
		{
			name: "image is used",
			image: &Info{
				Path:   "/foo/bar",
				usedBy: []string{"container_id"},
			},
			expectError: ErrIsUsed,
		},
		{
			name: "all ok",
			image: &Info{
				Path: f.Name(),
			},
			expectError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err = tc.image.Remove()
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
				ID: "7b0178cb4bac7227f83a56d62d3fdf9900645b6d53578aaad25a7df61ae15b39",
				Ref: &Reference{
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
	tt := []struct {
		name   string
		input  string
		expect *Info
	}{
		{
			name: "all filled",
			input: `
				{
					"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"size":741376,
					"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"ref":{
						"uri":"docker.io",
						"tags":["busybox:1.28"],
						"digests":null
					},
					"ociConfig":{
						"User":"sasha",
						"WorkingDir":"/opt/go",
						"Cmd":["./my-server"]
					}
				}`,
			expect: &Info{
				ID:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Size:   741376,
				Path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"busybox:1.28"},
				},
				OciConfig: &specs.ImageConfig{
					User:       "sasha",
					Cmd:        []string{"./my-server"},
					WorkingDir: "/opt/go",
				},
			},
		},
		{
			name: "no oci config",
			input: `
				{
					"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"size":741376,
					"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"ref":{
						"uri":"docker.io",
						"tags":["busybox:1.28"],
						"digests":null
					}
				}`,
			expect: &Info{
				ID:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Size:   741376,
				Path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"busybox:1.28"},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var info *Info
			err := json.Unmarshal([]byte(tc.input), &info)
			require.NoError(t, err, "could not unmarshal image")
			require.Equal(t, tc.expect, info)
		})
	}
}

func TestInfo_MarshalJSON(t *testing.T) {
	tt := []struct {
		name   string
		input  *Info
		expect string
	}{
		{
			name: "all filled",
			input: &Info{
				ID:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Size:   741376,
				Path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"busybox:1.28"},
				},
				OciConfig: &specs.ImageConfig{
					User:       "sasha",
					Cmd:        []string{"./my-server"},
					WorkingDir: "/opt/go",
				},
				usedBy: []string{"should-not-marshal"},
			},
			expect: `
				{
					"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"size":741376,
					"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"ref":{
						"uri":"docker.io",
						"tags":["busybox:1.28"],
						"digests":null
					},
					"ociConfig":{
						"User":"sasha",
						"WorkingDir":"/opt/go",
						"Cmd":["./my-server"]
					}
				}`,
		},
		{
			name: "no oci config",
			input: &Info{
				ID:     "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Sha256: "0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Size:   741376,
				Path:   "/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
				Ref: &Reference{
					uri:  singularity.DockerDomain,
					tags: []string{"busybox:1.28"},
				},
				usedBy: []string{"should-not-marshal"},
			},
			expect: `
				{
					"id":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"sha256":"0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"size":741376,
					"path":"/var/lib/singularity/0d408f32cc56b16509f30ae3dfa56ffb01269b2100036991d49af645a7b717a0",
					"ref":{
						"uri":"docker.io",
						"tags":["busybox:1.28"],
						"digests":null
					}
				}`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := json.Marshal(tc.input)
			require.NoError(t, err, "could not marshal image")
			require.JSONEq(t, tc.expect, string(res))
		})
	}
}
