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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/index"
	"github.com/sylabs/cri/pkg/singularity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const registryInfoFile = "registry.json"

// SingularityRegistry implements k8s ImageService interface.
type SingularityRegistry struct {
	storage string // path to image storage without trailing slash
	images  *index.ImageIndex

	m        sync.Mutex
	infoFile *os.File
}

// NewSingularityRegistry initializes and returns SingularityRuntime.
// Singularity must be installed on the host otherwise it will return an error.
func NewSingularityRegistry(storePath string, index *index.ImageIndex) (*SingularityRegistry, error) {
	_, err := exec.LookPath(singularity.RuntimeName)
	if err != nil {
		return nil, fmt.Errorf("could not find %s on this machine: %v", singularity.RuntimeName, err)
	}

	storePath, err = filepath.Abs(storePath)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute storage directory path: %v", err)
	}

	registry := SingularityRegistry{
		storage: storePath,
		images:  index,
	}

	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, fmt.Errorf("could not create storage directory: %v", err)
	}
	registry.infoFile, err = os.OpenFile(filepath.Join(storePath, registryInfoFile), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not open registry backup file: %v", err)
	}
	err = registry.loadInfo()
	if err != nil {
		return nil, err
	}
	return &registry, nil
}

// PullImage pulls an image with authentication config.
func (s *SingularityRegistry) PullImage(ctx context.Context, req *k8s.PullImageRequest) (*k8s.PullImageResponse, error) {
	ref, err := image.ParseRef(req.Image.Image)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not parse image reference: %v", err)
	}
	info, err := image.Pull(s.storage, ref)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not pull image: %v", err)
	}
	if err := info.Verify(); err != nil {
		info.Remove()
		return nil, status.Errorf(codes.InvalidArgument, "could not verify image: %v", err)
	}
	if err = s.images.Add(info); err != nil {
		info.Remove()
		return nil, status.Errorf(codes.Internal, "could not index image: %v", err)
	}
	if err = s.dumpInfo(); err != nil {
		log.Printf("could not dump registry info: %v", err)
	}
	return &k8s.PullImageResponse{
		ImageRef: info.ID(),
	}, nil
}

// RemoveImage removes the image.
// This call is idempotent, and does not return an error if the image has already been removed.
func (s *SingularityRegistry) RemoveImage(ctx context.Context, req *k8s.RemoveImageRequest) (*k8s.RemoveImageResponse, error) {
	info, err := s.images.Find(req.Image.Image)
	if err == index.ErrImageNotFound {
		return &k8s.RemoveImageResponse{}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not find image: %v", err)
	}
	if err := info.Remove(); err != nil {
		return nil, status.Errorf(codes.Internal, "could not remove image: %v", err)
	}
	if err := s.images.Remove(info.ID()); err != nil {
		return nil, status.Errorf(codes.Internal, "could not remove image from index: %v", err)
	}
	if err = s.dumpInfo(); err != nil {
		log.Printf("could not dump registry info: %v", err)
	}
	return &k8s.RemoveImageResponse{}, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to nil.
func (s *SingularityRegistry) ImageStatus(ctx context.Context, req *k8s.ImageStatusRequest) (*k8s.ImageStatusResponse, error) {
	info, err := s.images.Find(req.Image.Image)
	if err == index.ErrImageNotFound {
		return &k8s.ImageStatusResponse{}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not find image: %v", err)
	}
	return &k8s.ImageStatusResponse{
		Image: &k8s.Image{
			Id:          info.ID(),
			RepoTags:    info.Ref().Tags(),
			RepoDigests: info.Ref().Digests(),
			Size_:       info.Size(),
		},
	}, nil
}

// ListImages lists existing images.
func (s *SingularityRegistry) ListImages(ctx context.Context, req *k8s.ListImagesRequest) (*k8s.ListImagesResponse, error) {
	var imgs []*k8s.Image
	appendToResult := func(info *image.Info) {
		if info.Matches(req.Filter) {
			imgs = append(imgs, &k8s.Image{
				Id:          info.ID(),
				RepoTags:    info.Ref().Tags(),
				RepoDigests: info.Ref().Digests(),
				Size_:       info.Size(),
			})
		}
	}
	s.images.Iterate(appendToResult)
	return &k8s.ListImagesResponse{
		Images: imgs,
	}, nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *SingularityRegistry) ImageFsInfo(context.Context, *k8s.ImageFsInfoRequest) (*k8s.ImageFsInfoResponse, error) {
	mount, err := mountPoint(s.storage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get storage mount point: %v", err)
	}

	storeDir, err := os.Open(s.storage)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not open image storage: %v", err)
	}
	defer storeDir.Close()

	fi, err := storeDir.Stat()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get storage info: %v", err)
	}

	fii, err := storeDir.Readdir(-1)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not read image storage info: %v", err)
	}

	var inodes uint64
	var bytes int64
	// assume no sub directories there as we store images our particular way
	for _, fi := range fii {
		inodes++
		bytes += fi.Size()
	}
	// add directory info as well
	inodes++
	bytes += fi.Size()

	fsUsage := &k8s.FilesystemUsage{
		Timestamp: time.Now().UnixNano(),
		FsId: &k8s.FilesystemIdentifier{
			Mountpoint: mount,
		},
		UsedBytes: &k8s.UInt64Value{
			Value: uint64(bytes),
		},
		InodesUsed: &k8s.UInt64Value{
			Value: inodes,
		},
	}

	return &k8s.ImageFsInfoResponse{
		ImageFilesystems: []*k8s.FilesystemUsage{fsUsage},
	}, nil
}

// loadInfo reads backup file and restores registry according to it.
func (s *SingularityRegistry) loadInfo() error {
	s.m.Lock()
	defer s.m.Unlock()

	_, err := s.infoFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek registry info file: %v", err)
	}
	dec := json.NewDecoder(s.infoFile)

	// while the array contains values
	for dec.More() {
		var info *image.Info
		// decode an array value (Message)
		err := dec.Decode(&info)
		if err != nil {
			return fmt.Errorf("could not decode image: %v", err)
		}
		err = s.images.Add(info)
		if err != nil {
			return fmt.Errorf("could not add decoded image to index: %v", err)
		}
	}

	return nil
}

// dumpInfo dumps registry into backup file.
func (s *SingularityRegistry) dumpInfo() error {
	s.m.Lock()
	defer s.m.Unlock()

	_, err := s.infoFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek registry info file: %v", err)
	}
	err = s.infoFile.Truncate(0)
	if err != nil {
		return fmt.Errorf("could not reset file: %v", err)
	}
	enc := json.NewEncoder(s.infoFile)
	encodeToFile := func(info *image.Info) {
		err = enc.Encode(info)
	}
	s.images.Iterate(encodeToFile)
	if err != nil {
		return fmt.Errorf("could not encode image  %v", err)
	}
	return nil
}

// mountPoint parses mountinfo and returns the path of the parent
// mount point where provided path is mounted in
func mountPoint(path string) (string, error) {
	const (
		mountInfoPath = "/proc/self/mountinfo"
		defaultRoot   = "/"
	)

	p, err := os.Open(mountInfoPath)
	if err != nil {
		return "", fmt.Errorf("could not open %s: %v", mountInfoPath, err)
	}
	defer p.Close()

	var mountPoints []string
	scanner := bufio.NewScanner(p)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		mountPoints = append(mountPoints, fields[4])
	}

	for path != defaultRoot {
		for _, point := range mountPoints {
			if point == path {
				return point, nil
			}
		}
		path = filepath.Dir(path)
	}

	return defaultRoot, nil
}
