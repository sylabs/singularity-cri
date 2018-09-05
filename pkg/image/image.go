// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/sylabs/cri/pkg/singularity"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const registryInfoFile = "registry.json"

// SingularityRegistry implements k8s ImageService interface.
type SingularityRegistry struct {
	location string // path to directory without trailing slash

	m        sync.RWMutex
	refToID  map[string]string
	idToInfo map[string]imageInfo
	infoFile *os.File
}

// NewSingularityRegistry initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
func NewSingularityRegistry(storePath string) (*SingularityRegistry, error) {
	_, err := exec.LookPath(singularity.RuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s daemon on this machine: %v", singularity.RuntimeName, err)
	}

	storePath, err = filepath.Abs(storePath)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute storage directory path: %v", err)
	}

	registry := SingularityRegistry{
		location: storePath,
		refToID:  make(map[string]string),
		idToInfo: make(map[string]imageInfo),
	}
	registry.infoFile, err = os.OpenFile(registry.filePath(registryInfoFile), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not open registry file: %v", err)
	}
	err = registry.loadInfo()
	if err != nil {
		return nil, fmt.Errorf("could not load registry backup info: %v", err)
	}
	return &registry, nil
}

// ListImages lists existing images.
func (s *SingularityRegistry) ListImages(ctx context.Context, req *k8s.ListImagesRequest) (*k8s.ListImagesResponse, error) {
	// todo set uid or username

	imgs := make([]*k8s.Image, 0, len(s.idToInfo))
	s.m.RLock()
	for id, info := range s.idToInfo {
		img := &k8s.Image{
			Id:          id,
			RepoTags:    info.Tags,
			RepoDigests: info.Digests,
			Size_:       info.Size,
		}
		if matches(img, req.Filter) {
			imgs = append(imgs, img)
		}
	}
	s.m.RUnlock()

	return &k8s.ListImagesResponse{
		Images: imgs,
	}, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to nil.
func (s *SingularityRegistry) ImageStatus(ctx context.Context, req *k8s.ImageStatusRequest) (*k8s.ImageStatusResponse, error) {
	// todo set uid or username

	id := req.Image.Image
	s.m.RLock()
	info, ok := s.idToInfo[id]
	if !ok {
		id = s.refToID[normalizedImageRef(req.Image.Image)]
		info, ok = s.idToInfo[id]
	}
	s.m.RUnlock()

	if !ok {
		return &k8s.ImageStatusResponse{}, nil
	}

	return &k8s.ImageStatusResponse{
		Image: &k8s.Image{
			Id:          id,
			RepoTags:    info.Tags,
			RepoDigests: info.Digests,
			Size_:       info.Size,
		},
	}, nil
}

// PullImage pulls an image with authentication config.
func (s *SingularityRegistry) PullImage(ctx context.Context, req *k8s.PullImageRequest) (*k8s.PullImageResponse, error) {
	info, err := parseImageRef(req.Image.Image)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %v", err)
	}

	randID := randomString()
	pullPath := s.pullPath(randID)

	err = pullImage(req.Auth, pullPath, info)
	if err != nil {
		removeOrLog(pullPath)
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	pulled, err := os.Open(pullPath)
	if err != nil {
		removeOrLog(pullPath)
		return nil, fmt.Errorf("could not open pulled image: %v", err)
	}

	fi, err := pulled.Stat()
	if err != nil {
		removeOrLog(pullPath)
		return nil, fmt.Errorf("could not fetch file info: %v", err)
	}

	info.Size = uint64(fi.Size())

	h := sha256.New()
	_, err = io.Copy(h, pulled)
	if err != nil {
		removeOrLog(pullPath)
		return nil, fmt.Errorf("could not get pulled image digest: %v", err)
	}

	id := fmt.Sprintf("%x", h.Sum(nil))
	s.m.RLock()
	oldInfo := s.idToInfo[id]
	s.m.RUnlock()

	info.Tags = mergeStrSlice(oldInfo.Tags, info.Tags)
	info.Digests = mergeStrSlice(oldInfo.Digests, info.Digests)

	err = os.Rename(pullPath, s.filePath(id))
	if err != nil {
		return nil, fmt.Errorf("could not save pulled image: %v", err)
	}

	s.m.Lock()
	for _, tag := range info.Tags {
		oldId := s.refToID[tag]
		if oldId != "" && oldId != id {
			oldInfo = s.idToInfo[oldId]
			oldInfo.Tags = removeFromSlice(oldInfo.Tags, tag)
			s.idToInfo[oldId] = oldInfo
		}
		s.refToID[tag] = id
	}
	for _, digest := range info.Digests {
		oldDigest := s.refToID[digest]
		if oldDigest != "" && oldDigest != id {
			oldInfo = s.idToInfo[oldDigest]
			oldInfo.Digests = removeFromSlice(oldInfo.Digests, digest)
			s.idToInfo[oldDigest] = oldInfo
		}
		s.refToID[digest] = id
	}
	s.idToInfo[id] = info
	err = s.dumpInfo()
	s.m.Unlock()

	if err != nil {
		log.Printf("could not dump registry info: %v", err)
	}
	return &k8s.PullImageResponse{
		ImageRef: id,
	}, nil
}

// RemoveImage removes the image.
// This call is idempotent, and does not return an error if the image has already been removed.
func (s *SingularityRegistry) RemoveImage(ctx context.Context, req *k8s.RemoveImageRequest) (*k8s.RemoveImageResponse, error) {
	id := req.Image.Image
	s.m.RLock()
	info, ok := s.idToInfo[id]
	if !ok {
		id = s.refToID[normalizedImageRef(req.Image.Image)]
		info, ok = s.idToInfo[id]
	}
	s.m.RUnlock()

	if ok {
		s.m.Lock()
		err := os.Remove(s.filePath(id))
		if err != nil {
			s.m.Unlock()
			return nil, err
		}
		for _, tag := range info.Tags {
			delete(s.refToID, tag)
		}
		for _, digest := range info.Digests {
			delete(s.refToID, digest)
		}
		delete(s.idToInfo, id)
		err = s.dumpInfo()
		s.m.Unlock()

		if err != nil {
			log.Printf("could not dump registry info: %v", err)
		}
	}
	return &k8s.RemoveImageResponse{}, nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *SingularityRegistry) ImageFsInfo(context.Context, *k8s.ImageFsInfoRequest) (*k8s.ImageFsInfoResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *SingularityRegistry) loadInfo() error {
	_, err := s.infoFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek registry info file: %v", err)
	}
	err = json.NewDecoder(s.infoFile).Decode(&s.idToInfo)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}

	for id, info := range s.idToInfo {
		for _, tag := range info.Tags {
			s.refToID[tag] = id
		}
		for _, digest := range info.Digests {
			s.refToID[digest] = id
		}
	}
	return nil
}

func (s *SingularityRegistry) dumpInfo() error {
	_, err := s.infoFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek registry info file: %v", err)
	}
	err = s.infoFile.Truncate(0)
	if err != nil {
		return fmt.Errorf("could not reset file: %v", err)
	}
	return json.NewEncoder(s.infoFile).Encode(s.idToInfo)
}

func (s *SingularityRegistry) filePath(id string) string {
	return filepath.Join(s.location, id)
}

func (s *SingularityRegistry) pullPath(id string) string {
	return filepath.Join(s.location, "."+id)
}
