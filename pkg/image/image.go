// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/sylabs/cri/pkg/singularity"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SingularityRegistry implements k8s ImageService interface.
type SingularityRegistry struct {
	location string // path to directory without trailing slash

	m        sync.RWMutex
	registry map[string]k8s.Image // key:name value:info
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
	}

	err = registry.indexStorage()
	if err != nil {
		return nil, fmt.Errorf("could not index storage directory: %v", err)
	}
	return &registry, nil
}

// ListImages lists existing images.
func (s *SingularityRegistry) ListImages(ctx context.Context, req *k8s.ListImagesRequest) (*k8s.ListImagesResponse, error) {
	// todo apply filter
	imgs := make([]*k8s.Image, 0, len(s.registry))
	s.m.RLock()
	defer s.m.RUnlock()
	for _, info := range s.registry {
		// todo set uid or username
		infoCopy := info
		imgs = append(imgs, &infoCopy)
	}

	return &k8s.ListImagesResponse{
		Images: imgs,
	}, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to nil.
func (s *SingularityRegistry) ImageStatus(ctx context.Context, req *k8s.ImageStatusRequest) (*k8s.ImageStatusResponse, error) {
	// todo add meta information on verbose call
	s.m.RLock()
	img, ok := s.registry[req.Image.Image]
	s.m.RUnlock()
	if !ok {
		return &k8s.ImageStatusResponse{}, nil
	}
	return &k8s.ImageStatusResponse{
		Image: &img,
	}, nil
}

// PullImage pulls an image with authentication config.
func (s *SingularityRegistry) PullImage(ctx context.Context, req *k8s.PullImageRequest) (*k8s.PullImageResponse, error) {
	info, err := parseImageRef(req.Image.Image)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %v", err)
	}
	s.m.RLock()
	_, ok := s.registry[info.Id()]
	s.m.RUnlock()
	if ok {
		return &k8s.PullImageResponse{}, nil
	}

	err = info.Pull(req.Auth, s.location)
	if err != nil {
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	fi, err := os.Stat(s.filepath(info.Id()))
	if err != nil {
		return nil, err
	}
	size := uint64(fi.Size())

	s.m.Lock()
	s.registry[req.Image.Image] = k8s.Image{
		Id:          "",
		RepoTags:    info.Tags(),
		RepoDigests: info.Digests(),
		Size_:       size,
	}
	s.m.Unlock()

	return &k8s.PullImageResponse{
		ImageRef: info.Id(),
	}, nil
}

// RemoveImage removes the image.
// This call is idempotent, and does not return an error if the image has already been removed.
func (s *SingularityRegistry) RemoveImage(ctx context.Context, req *k8s.RemoveImageRequest) (*k8s.RemoveImageResponse, error) {
	s.m.RLock()
	_, ok := s.registry[req.Image.Image]
	s.m.RUnlock()

	if ok {
		s.m.Lock()
		delete(s.registry, req.Image.Image)
		s.m.Unlock()

		err := os.Remove(s.filepath(req.Image.Image))
		if err != nil {
			return nil, err
		}
	}

	return &k8s.RemoveImageResponse{}, nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *SingularityRegistry) ImageFsInfo(context.Context, *k8s.ImageFsInfoRequest) (*k8s.ImageFsInfoResponse, error) {
	return &k8s.ImageFsInfoResponse{}, nil
}

func (s *SingularityRegistry) indexStorage() error {
	// todo check empty path
	// todo check files extension
	files, err := ioutil.ReadDir(s.location)
	if err != nil {
		return fmt.Errorf("could not read store directory: %v", err)
	}

	s.registry = make(map[string]k8s.Image, len(files))
	for _, file := range files {
		s.registry[file.Name()] = k8s.Image{
			Id:    file.Name(),
			Size_: uint64(file.Size()),
		}
	}
	return nil
}

func (s *SingularityRegistry) filepath(name string) string {
	return filepath.Join(s.location, name)
}
