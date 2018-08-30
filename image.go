// Copyright (c) 2018, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package sycri

import (
	"context"
	"log"

	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// SignularityImageService implements k8s ImageService interface.
type SignularityImageService struct {
}

func (s *SignularityImageService) ListImages(context.Context, *v1alpha2.ListImagesRequest) (*v1alpha2.ListImagesResponse, error) {
	log.Println("SignularityImageService.ListImages")
	return &v1alpha2.ListImagesResponse{}, nil
}

func (s *SignularityImageService) ImageStatus(context.Context, *v1alpha2.ImageStatusRequest) (*v1alpha2.ImageStatusResponse, error) {
	log.Println("SignularityImageService.ImageStatus")
	return &v1alpha2.ImageStatusResponse{}, nil
}

func (s *SignularityImageService) PullImage(context.Context, *v1alpha2.PullImageRequest) (*v1alpha2.PullImageResponse, error) {
	log.Println("SignularityImageService.PullImage")
	return &v1alpha2.PullImageResponse{}, nil
}

func (s *SignularityImageService) RemoveImage(context.Context, *v1alpha2.RemoveImageRequest) (*v1alpha2.RemoveImageResponse, error) {
	log.Println("SignularityImageService.RemoveImage")
	return &v1alpha2.RemoveImageResponse{}, nil
}

func (s *SignularityImageService) ImageFsInfo(context.Context, *v1alpha2.ImageFsInfoRequest) (*v1alpha2.ImageFsInfoResponse, error) {
	log.Println("SignularityImageService.ImageFsInfo")
	return &v1alpha2.ImageFsInfoResponse{}, nil
}
