// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package image

import (
	"context"
	"os"

	sy "github.com/singularityware/singularity/src/pkg/library/client"
	"github.com/sylabs/cri/pkg/singularity"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SingularityRegistry implements k8s ImageService interface.
type SingularityRegistry struct {
	location string
	registry map[string]*v1alpha2.Image // key:name value:info
}

// NewSingularityRegistry initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
func NewSingularityRegistry() (*SingularityRegistry, error) {
	// todo scan folder on start
	// todo synchronize registry
	return &SingularityRegistry{
		location: "./images/", // todo make an argument
		registry: make(map[string]*v1alpha2.Image),
	}, nil
}

// ListImages lists existing images.
func (s *SingularityRegistry) ListImages(ctx context.Context, req *v1alpha2.ListImagesRequest) (*v1alpha2.ListImagesResponse, error) {
	imgs := make([]*v1alpha2.Image, len(s.registry), 0)
	for _, info := range s.registry {
		// todo set uid?
		imgs = append(imgs, info)
	}

	return &v1alpha2.ListImagesResponse{
		Images: imgs,
	}, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to nil.
func (s *SingularityRegistry) ImageStatus(ctx context.Context, req *v1alpha2.ImageStatusRequest) (*v1alpha2.ImageStatusResponse, error) {
	return &v1alpha2.ImageStatusResponse{
		Image: s.registry[req.Image.Image],
	}, nil
}

// PullImage pulls an image with authentication config.
func (s *SingularityRegistry) PullImage(ctx context.Context, req *v1alpha2.PullImageRequest) (*v1alpha2.PullImageResponse, error) {
	var token string
	if req.Auth != nil {
		token = req.Auth.RegistryToken
	}

	err := sy.DownloadImage(s.location+req.Image.Image, req.Image.Image, singularity.LibraryURL, true, token)
	if err != nil {
		return nil, err
	}

	fi, err := os.Stat(s.location + req.Image.Image)
	if err != nil {
		return nil, err
	}

	s.registry[req.Image.Image] = &v1alpha2.Image{
		Id:    req.Image.Image,
		Size_: uint64(fi.Size()),
		// todo set uid?
	}
	return &v1alpha2.PullImageResponse{
		ImageRef: req.Image.Image,
	}, nil

	//var stderr bytes.Buffer
	//pullCmd := exec.Command(s.singularity, "pull", "library://"+req.Image.Image) // todo configure urI?
	//pullCmd.Stderr = &stderr
	//err := pullCmd.Run()
	//if err != nil {
	//	return nil, err
	//}
	//
	//logs := stderr.String()
	//i := strings.LastIndexByte(logs, ' ')
	//imgRef := req.Image.Image
	//if i != -1 {
	//	imgRef = logs[i+1:]
	//}
	//
	//return &v1alpha2.PullImageResponse{
	//	ImageRef: imgRef, // todo path
	//}, nil
}

// RemoveImage removes the image.
// This call is idempotent, and does not return an error if the image has already been removed.
func (s *SingularityRegistry) RemoveImage(ctx context.Context, req *v1alpha2.RemoveImageRequest) (*v1alpha2.RemoveImageResponse, error) {
	if _, ok := s.registry[req.Image.Image]; !ok {
		return &v1alpha2.RemoveImageResponse{}, nil
	}

	err := os.Remove(s.location + req.Image.Image)
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
