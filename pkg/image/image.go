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
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/sylabs/cri/pkg/rand"
	"github.com/sylabs/cri/pkg/singularity"
	"github.com/sylabs/sif/pkg/sif"
	library "github.com/sylabs/singularity/pkg/client/library"
	"github.com/sylabs/singularity/pkg/signing"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	// IDLen reflects number of symbols in image unique ID.
	IDLen = 64
)

// Info represents image stored on the host filesystem.
type Info struct {
	id     string
	sha256 string
	size   uint64
	path   string
	ref    *Reference
}

// ID returns id of an image.
func (i *Info) ID() string {
	return i.id
}

// SetID sets desired image id. Should be used when
// default ID (image sha256 checksum) doesn't fit needs.
func (i *Info) SetID(id string) {
	i.id = id
}

// Path returns path to image file.
func (i *Info) Path() string {
	return i.path
}

// Size returns image size in bytes.
func (i *Info) Size() uint64 {
	return i.size
}

// Ref returns associated image reference.
func (i *Info) Ref() *Reference {
	return i.ref
}

// SetRef sets associated image reference. Should be used
// in rare cases when one wishes to override Reference that
// was used to pull image.
func (i *Info) SetRef(ref *Reference) {
	i.ref = ref
}

// MarshalJSON marshals Info into a valid JSON.
func (i *Info) MarshalJSON() ([]byte, error) {
	jsonInfo := struct {
		ID     string     `json:"id"`
		Sha256 string     `json:"sha256"`
		Size   uint64     `json:"size"`
		Path   string     `json:"path"`
		Ref    *Reference `json:"ref"`
	}{
		ID:     i.id,
		Sha256: i.sha256,
		Size:   i.size,
		Path:   i.path,
		Ref:    i.ref,
	}
	return json.Marshal(jsonInfo)
}

// UnmarshalJSON unmarshals a valid Info JSON into an object.
func (i *Info) UnmarshalJSON(data []byte) error {
	jsonInfo := struct {
		ID     string     `json:"id"`
		Sha256 string     `json:"sha256"`
		Size   uint64     `json:"size"`
		Path   string     `json:"path"`
		Ref    *Reference `json:"ref"`
	}{}
	err := json.Unmarshal(data, &jsonInfo)
	i.id = jsonInfo.ID
	i.sha256 = jsonInfo.Sha256
	i.size = jsonInfo.Size
	i.path = jsonInfo.Path
	i.ref = jsonInfo.Ref
	return err
}

// Pull pulls image referenced by ref and saves it to the passed location.
func Pull(location string, ref *Reference) (img *Info, err error) {
	pullPath := filepath.Join(location, "."+rand.GenerateID(64))
	glog.V(8).Infof("Pulling to temporary file %s", pullPath)
	defer func() {
		if err != nil {
			if err := os.Remove(pullPath); err != nil {
				glog.Errorf("Could not remove temporary image file: %v", err)
			}
		}
	}()

	pullURL := strings.TrimPrefix(ref.String(), ref.uri+"/")
	switch ref.uri {
	case singularity.LibraryDomain:
		err = library.DownloadImage(pullPath, pullURL, singularity.LibraryURL, true, "")
	case singularity.DockerDomain:
		remote := fmt.Sprintf("%s://%s", singularity.DockerProtocol, pullURL)
		var errMsg bytes.Buffer
		buildCmd := exec.Command(singularity.RuntimeName, "build", "-F", pullPath, remote)
		buildCmd.Stderr = &errMsg
		buildCmd.Stdout = ioutil.Discard
		err = buildCmd.Run()
		if err != nil {
			err = fmt.Errorf("could not build image: %s", &errMsg)
		}
	default:
		err = fmt.Errorf("unknown image registry: %s", ref.uri)
	}
	if err != nil {
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	pulled, err := os.Open(pullPath)
	if err != nil {
		return nil, fmt.Errorf("could not open pulled image: %v", err)
	}

	fi, err := pulled.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not fetch file info: %v", err)
	}

	h := sha256.New()
	_, err = io.Copy(h, pulled)
	if err != nil {
		return nil, fmt.Errorf("could not get pulled image digest: %v", err)
	}
	checksum := fmt.Sprintf("%x", h.Sum(nil))

	err = pulled.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close pulled image: %v", err)
	}

	path := filepath.Join(location, checksum)
	glog.V(8).Infof("Renaming %s to %s", pullPath, path)
	err = os.Rename(pullPath, path)
	if err != nil {
		return nil, fmt.Errorf("could not save pulled image: %v", err)
	}

	return &Info{
		id:     checksum,
		sha256: checksum,
		size:   uint64(fi.Size()),
		path:   path,
		ref:    ref,
	}, nil
}

// Remove removes image from the host filesystem.
func (i *Info) Remove() error {
	err := os.Remove(i.path)
	if err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}
	return nil
}

// Verify verifies image signatures.
func (i *Info) Verify() error {
	if i.ref.URI() == singularity.DockerDomain {
		return nil
	}
	fimg, err := sif.LoadContainer(i.path, true)
	if err != nil {
		return fmt.Errorf("failed to load SIF image: %v", err)
	}
	defer fimg.UnloadContainer()

	err = signing.Verify(i.path, singularity.KeysServer, 0, false, "", true)

	noSignatures := strings.Contains(err.Error(), "no signatures found")
	if noSignatures {
		glog.V(4).Infof("Image %s is not signed", i.ref)
	}
	if err != nil && !noSignatures {
		return fmt.Errorf("SIF verification failed: %v", err)
	}
	return nil
}

// Matches tests image against passed filter and returns true if it matches.
func (i *Info) Matches(filter *k8s.ImageFilter) bool {
	if filter == nil || filter.Image == nil {
		return true
	}
	ref := filter.Image.Image
	if strings.HasPrefix(i.ID(), ref) {
		return true
	}
	for _, tag := range i.ref.tags {
		if strings.HasPrefix(tag, ref) {
			return true
		}
	}
	for _, digest := range i.ref.digests {
		if strings.HasPrefix(digest, ref) {
			return true
		}
	}
	return false
}
