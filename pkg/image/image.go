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
	"context"
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
	library "github.com/sylabs/scs-library-client/client"
	"github.com/sylabs/singularity-cri/pkg/rand"
	"github.com/sylabs/singularity-cri/pkg/singularity"
	"github.com/sylabs/singularity-cri/pkg/slice"
	"github.com/sylabs/singularity/pkg/image"
	"github.com/sylabs/singularity/pkg/signing"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	// IDLen reflects number of symbols in image unique ID.
	IDLen = 64
)

var (
	// ErrIsUsed notifies that image is currently being used by someone.
	ErrIsUsed = fmt.Errorf("image is being used")
	// ErrNotFound notifies that image is not found thus cannot be pulled.
	ErrNotFound = fmt.Errorf("image is not found")
	// ErrNotLibrary is used when user tried to get library image metadata but
	// provided non library image reference.
	ErrNotLibrary = fmt.Errorf("not library image")
)

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
func Pull(ctx context.Context, location string, ref *Reference, auth *k8s.AuthConfig) (img *Info, err error) {
	pullPath := filepath.Join(location, "."+rand.GenerateID(64))
	glog.V(5).Infof("Pulling %s to temporary file %s", ref, pullPath)
	defer func() {
		if err != nil {
			if err := os.Remove(pullPath); err != nil && !os.IsNotExist(err) {
				glog.Errorf("Could not remove %s: %v", pullPath, err)
			}
		}
	}()

	pullURL := strings.TrimPrefix(ref.String(), ref.URI()+"/")
	switch ref.URI() {
	case singularity.LibraryDomain:
		config := &library.Config{
			BaseURL:   auth.GetServerAddress(),
			AuthToken: auth.GetRegistryToken(),
		}
		client, err := library.NewClient(config)
		if err != nil {
			return nil, fmt.Errorf("could not create library client: %v", err)
		}
		w, err := os.Create(pullPath)
		if err != nil {
			return nil, fmt.Errorf("could not create file to pull image: %v", err)
		}
		parts := strings.Split(pullURL, ":")
		// don't check index out of range since we add :latest by default when parsing ref
		err = client.DownloadImage(ctx, w, parts[0], parts[1], nil)
		_ = w.Close()
		if err != nil {
			return nil, fmt.Errorf("could not pull library image: %v", err)
		}
	case singularity.DockerDomain:
		remote := fmt.Sprintf("%s://%s", singularity.DockerProtocol, pullURL)
		var errMsg bytes.Buffer
		buildCmd := exec.CommandContext(ctx, singularity.RuntimeName, "build", "-F", pullPath, remote)
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
	glog.V(5).Infof("Renaming %s to %s", pullPath, path)
	err = os.Rename(pullPath, path)
	if err != nil {
		return nil, fmt.Errorf("could not save pulled image: %v", err)
	}

	ociConfig, err := fetchOCIConfig(path)
	if err != nil {
		glog.Errorf("Could not fetch OCI config for image %s: %v", ref, err)
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

// LibraryInfo queries remote library to get info about the image.
// If image is not found returns ErrNotFound. For references other than
// library returns ErrNotLibrary.
func LibraryInfo(ctx context.Context, ref *Reference, auth *k8s.AuthConfig) (*Info, error) {
	if ref.URI() != singularity.LibraryDomain {
		return nil, ErrNotLibrary
	}

	pullURL := strings.TrimPrefix(ref.String(), ref.URI()+"/")
	config := &library.Config{
		BaseURL:   auth.GetServerAddress(),
		AuthToken: auth.GetRegistryToken(),
	}
	client, err := library.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("could not create library client: %v", err)
	}
	img, err := client.GetImage(ctx, pullURL)
	if err == library.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not get library image info: %v", err)
	}

	// library API uses sha256 hash func and returns image hash in form sha256.<hash>
	// we need to trim it before it can be used
	id := strings.TrimPrefix(img.Hash, "sha256.")
	return &Info{
		ID:     id,
		Sha256: id,
		Size:   uint64(img.Size),
		Ref:    ref,
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

	_, err := signing.Verify(i.Path, singularity.KeysServer, 0, false, "", false, true)
	noSignatures := err != nil && strings.Contains(err.Error(), "no signatures found")
	if noSignatures {
		glog.V(2).Infof("Image %s is not signed", i.Ref)
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
