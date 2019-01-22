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

	"github.com/sylabs/cri/pkg/kube"
	"github.com/sylabs/cri/pkg/truncindex"
)

// ContainerIndex provides a convenient and thread-safe way for storing containers.
type ContainerIndex struct {
	indx *truncindex.TruncIndex
}

// NewContainerIndex returns new ContainerIndex ready to use.
func NewContainerIndex() *ContainerIndex {
	return &ContainerIndex{
		indx: truncindex.NewTruncIndex(kube.ContainerIDLen),
	}
}

// Find searches for container by its ID or prefix. This method may return error if
// prefix is not long enough to identify container uniquely.
func (i *ContainerIndex) Find(id string) (*kube.Container, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	cont, _ := item.(*kube.Container)
	return cont, nil
}

// Remove removes container from index if it present or does nothing otherwise.
func (i *ContainerIndex) Remove(id string) error {
	err := i.indx.Delete(id)
	if err == truncindex.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not remove container: %v", err)
	}
	return nil
}

// Add adds the given container. If container already exists it returns an error.
func (i *ContainerIndex) Add(cont *kube.Container) error {
	err := i.indx.Add(cont.ID(), cont)
	if err != nil {
		return fmt.Errorf("could not add container: %v", err)
	}
	return nil
}

// Iterate calls handler func on each container registered in index.
func (i *ContainerIndex) Iterate(handler func(*kube.Container)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*kube.Container))
	}
	i.indx.Iterate(innerIterate)
}
