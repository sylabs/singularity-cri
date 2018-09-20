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
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"

	"github.com/sylabs/cri/pkg/singularity"
	library "github.com/sylabs/singularity/src/pkg/library/client"
	shub "github.com/sylabs/singularity/src/pkg/shub/client"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type imageInfo struct {
	Origin  string
	Tags    []string
	Digests []string
	Size    uint64
}

func parseImageRef(ref string) (imageInfo, error) {
	uri := singularity.DockerProtocol
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		uri = image[:indx]
		image = image[indx+3:]
	}

	info := imageInfo{
		Origin: uri,
	}

	switch uri {
	case singularity.ShubProtocol:
		fallthrough
	case singularity.LibraryProtocol:
		if strings.Contains(image, "sha256.") {
			info.Digests = append(info.Digests, ref)
		} else {
			info.Tags = append(info.Tags, normalizedImageRef(ref))
		}
	case singularity.DockerProtocol:
		if strings.IndexByte(image, '@') != -1 {
			info.Digests = append(info.Digests, image)
		} else {
			info.Tags = append(info.Tags, normalizedImageRef(image))
		}
	default:
		return imageInfo{}, fmt.Errorf("unknown image registry: %s", uri)
	}

	return info, nil
}

func pullImage(_ *k8s.AuthConfig, path string, image imageInfo) error {
	var ref string
	if len(image.Tags) > 0 {
		ref = image.Tags[0]
	} else {
		ref = image.Digests[0]
	}

	switch uri := image.Origin; uri {
	case singularity.LibraryProtocol:
		return library.DownloadImage(path, ref, singularity.LibraryURL, true, "")
	case singularity.ShubProtocol:
		return shub.DownloadImage(path, ref, true)
	case singularity.DockerProtocol:
		remote := fmt.Sprintf("%s://%s", image.Origin, ref)
		buildCmd := exec.Command(singularity.RuntimeName, "build", "-F", path, remote)
		return buildCmd.Run()
	default:
		return fmt.Errorf("unknown image registry: %s", uri)
	}
}

// randomString returns random string of length 64 generated with math/rand.Read.
func randomString() string {
	buf := make([]byte, 32)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func removeOrLog(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Printf("could not remove temparary image file: %v", err)
	}
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
// It does not makes a copy of a passed slice.
func removeFromSlice(a []string, v string) []string {
	for i, str := range a {
		if str == v {
			return append(a[:i], a[i+1:]...)
		}
	}
	return a
}

// normalizedImageRef appends tag 'latest' if the passed ref
// does not have any tag or digest already.
func normalizedImageRef(ref string) string {
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		image = ref[indx+3:]
	}
	i := strings.LastIndexByte(image, ':')
	if i == -1 {
		return ref + ":latest"
	}
	return ref
}

func matches(image *k8s.Image, filter *k8s.ImageFilter) bool {
	if filter == nil || filter.Image == nil {
		return true
	}
	ref := filter.Image.Image
	if strings.HasPrefix(image.Id, ref) {
		return true
	}
	for _, tag := range image.RepoTags {
		if strings.HasPrefix(tag, ref) {
			return true
		}
	}
	for _, digest := range image.RepoDigests {
		if strings.HasPrefix(digest, ref) {
			return true
		}
	}
	return false
}
