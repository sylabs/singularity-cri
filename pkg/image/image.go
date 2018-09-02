// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	library "github.com/singularityware/singularity/src/pkg/library/client"
	shub "github.com/singularityware/singularity/src/pkg/shub/client"
	"github.com/sylabs/cri/pkg/singularity"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SingularityRegistry implements k8s ImageService interface.
type SingularityRegistry struct {
	singularity string
	location    string // path to directory without trailing slash

	m        sync.RWMutex
	registry map[string]v1alpha2.Image // key:name value:info
}

// NewSingularityRegistry initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
func NewSingularityRegistry(storePath string) (*SingularityRegistry, error) {
	s, err := exec.LookPath(singularity.RuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s daemon on this machine: %v", singularity.RuntimeName, err)
	}
	storePath, err = filepath.Abs(storePath)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute storing path: %v", err)
	}
	return &SingularityRegistry{
		singularity: s,
		location:    storePath,
		registry:    make(map[string]v1alpha2.Image),
	}, nil
}

// ListImages lists existing images.
func (s *SingularityRegistry) ListImages(ctx context.Context, req *v1alpha2.ListImagesRequest) (*v1alpha2.ListImagesResponse, error) {
	// todo apply filter
	imgs := make([]*v1alpha2.Image, len(s.registry), 0)
	s.m.RLock()
	defer s.m.RUnlock()
	for _, info := range s.registry {
		// todo set uid or username
		imgs = append(imgs, &info)
	}

	return &v1alpha2.ListImagesResponse{
		Images: imgs,
	}, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to nil.
func (s *SingularityRegistry) ImageStatus(ctx context.Context, req *v1alpha2.ImageStatusRequest) (*v1alpha2.ImageStatusResponse, error) {
	// todo add meta information on verbose call
	s.m.RLock()
	defer s.m.RUnlock()
	img := s.registry[req.Image.Image]
	return &v1alpha2.ImageStatusResponse{
		Image: &img,
	}, nil
}

// PullImage pulls an image with authentication config.
func (s *SingularityRegistry) PullImage(ctx context.Context, req *v1alpha2.PullImageRequest) (*v1alpha2.PullImageResponse, error) {
	uri := "docker"
	image := req.Image.Image
	indx := strings.Index(image, "://")
	if indx != -1 {
		uri = image[:indx]
		image = image[indx:]
	}

	var err error
	var img v1alpha2.Image
	switch uri {
	case "library":
		info := parseLibraryRef(image)
		name := info.filename()
		img.Id = name
		img.RepoTags = info.tags
		err = library.DownloadImage(s.filepath(name), info.ref, singularity.LibraryURL, false, "")
	case "shub":
		info, err := parseShubRef(image)
		if err != nil {
			return nil, fmt.Errorf("could not parse shub ref: %v", err)
		}
		name := info.filename()
		img.Id = name
		img.RepoTags = info.tags
		err = shub.DownloadImage(s.filepath(name), info.ref, false)
	case "docker":
		info := parseOCIRef(image)
		name := info.filename()
		img.Id = name
		img.RepoTags = info.tags
		buildCmd := exec.Command(s.singularity, "build", s.filepath(name), uri+"://"+image)
		err = buildCmd.Run()
	default:
		return nil, fmt.Errorf("unknown image registry: %s", uri)
	}
	if err != nil {
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	fi, err := os.Stat(s.filepath(img.Id))
	if err != nil {
		return nil, err
	}
	img.Size_ = uint64(fi.Size())

	s.registry[req.Image.Image] = img
	return &v1alpha2.PullImageResponse{
		ImageRef: img.Id,
	}, nil
}

// RemoveImage removes the image.
// This call is idempotent, and does not return an error if the image has already been removed.
func (s *SingularityRegistry) RemoveImage(ctx context.Context, req *v1alpha2.RemoveImageRequest) (*v1alpha2.RemoveImageResponse, error) {
	s.m.Lock()
	defer s.m.Unlock()

	// todo rlock + lock ?
	if _, ok := s.registry[req.Image.Image]; !ok {
		return &v1alpha2.RemoveImageResponse{}, nil
	}

	err := os.Remove(s.filepath(req.Image.Image))
	if err != nil {
		return nil, err
	}
	delete(s.registry, req.Image.Image)

	return &v1alpha2.RemoveImageResponse{}, nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *SingularityRegistry) ImageFsInfo(context.Context, *v1alpha2.ImageFsInfoRequest) (*v1alpha2.ImageFsInfoResponse, error) {
	return &v1alpha2.ImageFsInfoResponse{}, nil
}

func (s *SingularityRegistry) filepath(name string) string {
	return s.location + "/" + name
}
