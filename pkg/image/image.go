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
	"sync"

	"github.com/golang/glog"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sylabs/sif/pkg/sif"
	"github.com/sylabs/singularity-cri/pkg/rand"
	"github.com/sylabs/singularity-cri/pkg/singularity"
	"github.com/sylabs/singularity-cri/pkg/slice"
	library "github.com/sylabs/singularity/pkg/client/library"
	"github.com/sylabs/singularity/pkg/image"
	"github.com/sylabs/singularity/pkg/signing"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	// IDLen reflects number of symbols in image unique ID.
	IDLen = 64
)

// ErrIsUsed notifies that image is currently being used by someone.
var ErrIsUsed = fmt.Errorf("image is being used")

// Info represents image stored on the host filesystem.
type Info struct {
	ID        string             `json:"id"`
	Sha256    string             `json:"sha256"`
	Size      uint64             `json:"size"`
	Path      string             `json:"path"`
	Ref       *Reference         `json:"ref"`
	OciConfig *specs.ImageConfig `json:"ociConfig,omitempty"`

	mu     sync.RWMutex
	usedBy []string
}

// Borrow notifies that image is used by some container and should
// not be removed until Return with the same parameters is called.
// This method is thread-safe to use.
func (i *Info) Borrow(who string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.usedBy = slice.MergeString(i.usedBy, who)
}

// Return notifies that image is no longer used by a container and
// may be safely removed if no one else needs it anymore.
// This method is thread-safe to use.
func (i *Info) Return(who string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.usedBy = slice.RemoveFromString(i.usedBy, who)
}

// UsedBy returns list of container ids that use this image.
func (i *Info) UsedBy() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	usedBy := make([]string, len(i.usedBy))
	copy(usedBy, i.usedBy)
	return usedBy
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

	pullURL := strings.TrimPrefix(ref.String(), ref.URI()+"/")
	switch ref.URI() {
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
		err = fmt.Errorf("unknown image registry: %s", ref.URI())
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

	ociConfig, err := fetchOCIConfig(path)
	if err != nil {
		glog.Errorf("Could not fetch image's OCI config: %v", err)
	}

	return &Info{
		ID:        checksum,
		Sha256:    checksum,
		Size:      uint64(fi.Size()),
		Path:      path,
		Ref:       ref,
		OciConfig: ociConfig,
	}, nil
}

// Remove removes image from the host filesystem. It makes sure
// no one relies on image file and if this check fails it returns ErrIsUsed error.
func (i *Info) Remove() error {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if len(i.usedBy) > 0 {
		return ErrIsUsed
	}

	err := os.Remove(i.Path)
	if err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}
	return nil
}

// Verify verifies image signatures.
func (i *Info) Verify() error {
	if i.Ref.URI() == singularity.DockerDomain {
		return nil
	}
	fimg, err := sif.LoadContainer(i.Path, true)
	if err != nil {
		return fmt.Errorf("failed to load SIF image: %v", err)
	}
	defer fimg.UnloadContainer()

	err = signing.Verify(i.Path, singularity.KeysServer, 0, false, "", true)

	noSignatures := err != nil && strings.Contains(err.Error(), "no signatures found")
	if noSignatures {
		glog.V(4).Infof("Image %s is not signed", i.Ref)
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
	if strings.HasPrefix(i.ID, ref) {
		return true
	}
	for _, tag := range i.Ref.tags {
		if strings.HasPrefix(tag, ref) {
			return true
		}
	}
	for _, digest := range i.Ref.digests {
		if strings.HasPrefix(digest, ref) {
			return true
		}
	}
	return false
}

func fetchOCIConfig(imgPath string) (*specs.ImageConfig, error) {
	const ociConfigSection = "oci-config.json"

	img, err := image.Init(imgPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load SIF image %s: %v", imgPath, err)
	}
	defer img.File.Close()

	reader, err := image.NewSectionReader(img, ociConfigSection, -1)
	if err != nil {
		if err == image.ErrNoSection {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s section: %v", ociConfigSection, err)
	}

	var imgConfig specs.ImageConfig
	err = json.NewDecoder(reader).Decode(&imgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s section: %v", ociConfigSection, err)
	}

	return &imgConfig, nil
}
