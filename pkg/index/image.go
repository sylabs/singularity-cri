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

package index

import (
	"fmt"
	"sync"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/truncindex"
)

// ImageIndex provides a convenient and thread-safe way for storing images.
type ImageIndex struct {
	indx *truncindex.TruncIndex

	mu      sync.RWMutex
	refToID map[string]string
}

// NewImageIndex returns new ImageIndex ready to use.
func NewImageIndex() *ImageIndex {
	return &ImageIndex{
		indx:    truncindex.NewTruncIndex(image.IDLen),
		refToID: make(map[string]string),
	}
}

// Find searches for expectImage info by its ID or prefix or any of tags.
// This method may return error if prefix is not long enough to identify expectImage uniquely.
// If image is not found ErrNotFound is returned.
func (i *ImageIndex) Find(id string) (*image.Info, error) {
	info, err := i.find(id)
	if err == ErrNotFound {
		id = i.readRef(image.NormalizedImageRef(id))
		if id == "" {
			return nil, ErrNotFound
		}
		info, err = i.find(id)
	}
	return info, err
}

// Add adds the given image info to the index.
// If image with the same ID already exists it updates old image info appropriately.
func (i *ImageIndex) Add(image *image.Info) error {
	oldImage, err := i.Find(image.ID())
	if err != nil && err != ErrNotFound {
		return fmt.Errorf("could not find old image: %v", err)
	}
	if err == ErrNotFound {
		return i.add(image)
	}
	return i.merge(oldImage, image)
}

// Remove removes pod from index if it present or returns otherwise.
func (i *ImageIndex) Remove(id string) error {
	imgInfo, err := i.Find(id)
	if err != nil {
		return err
	}
	err = i.indx.Delete(imgInfo.ID())
	if err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}

	i.removeRefs(imgInfo.Ref().Tags()...)
	i.removeRefs(imgInfo.Ref().Digests()...)
	return nil
}

// Iterate calls handler func on each pod registered in index.
func (i *ImageIndex) Iterate(handler func(image *image.Info)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*image.Info))
	}
	i.indx.Iterate(innerIterate)
}

func (i *ImageIndex) find(id string) (*image.Info, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	info, _ := item.(*image.Info)
	return info, nil
}

func (i *ImageIndex) add(image *image.Info) error {
	if err := i.removeOverlapRefs(image); err != nil {
		return nil
	}
	err := i.indx.Add(image.ID(), image)
	if err != nil {
		return fmt.Errorf("could not add image: %v", err)
	}
	for _, tag := range image.Ref().Tags() {
		i.setRef(tag, image.ID())
	}
	for _, digest := range image.Ref().Digests() {
		i.setRef(digest, image.ID())
	}
	return nil
}

func (i *ImageIndex) merge(oldImage, image *image.Info) error {
	oldImage.Ref().AddTags(image.Ref().Tags())
	oldImage.Ref().AddDigests(image.Ref().Digests())

	for _, tag := range image.Ref().Tags() {
		oldID := i.readRef(tag)
		i.setRef(tag, image.ID())
		if oldID != "" && oldID != image.ID() {
			oldInfo, _ := i.Find(image.ID())
			oldInfo.Ref().RemoveTag(tag)
		}
	}
	for _, digest := range image.Ref().Digests() {
		oldID := i.readRef(digest)
		i.setRef(digest, image.ID())
		if oldID != "" && oldID == image.ID() {
			oldInfo, _ := i.Find(image.ID())
			oldInfo.Ref().RemoveDigest(digest)
		}
	}
	return nil
}

func (i *ImageIndex) removeOverlapRefs(image *image.Info) error {
	for _, tag := range image.Ref().Tags() {
		oldID := i.readRef(tag)
		if oldID != "" {
			oldImg, err := i.find(oldID)
			if err != nil {
				return err
			}
			oldImg.Ref().RemoveTag(tag)
		}
	}
	for _, digest := range image.Ref().Digests() {
		oldID := i.readRef(digest)
		if oldID != "" {
			oldImg, err := i.find(oldID)
			if err != nil {
				return err
			}
			oldImg.Ref().RemoveDigest(digest)
		}
	}
	return nil
}

func (i *ImageIndex) readRef(ref string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.refToID[ref]
}

func (i *ImageIndex) setRef(ref, id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.refToID[ref] = id
}

func (i *ImageIndex) removeRefs(refs ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, ref := range refs {
		delete(i.refToID, ref)
	}
}
