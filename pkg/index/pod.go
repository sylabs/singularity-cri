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

// PodIndex provides a convenient and thread-safe way for storing pods.
type PodIndex struct {
	indx *truncindex.TruncIndex
}

var (
	// ErrPodNotFound returned when pod is not found in index.
	ErrPodNotFound = fmt.Errorf("pod not found")
)

// NewPodIndex returns new PodIndex ready to use.
func NewPodIndex() *PodIndex {
	return &PodIndex{
		indx: truncindex.NewTruncIndex(kube.PodIDLen),
	}
}

// Find searches for pod by its ID or prefix. This method may return error if
// prefix is not long enough to identify pod uniquely.
func (i *PodIndex) Find(id string) (*kube.Pod, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrPodNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	pod, _ := item.(*kube.Pod)
	return pod, nil
}

// Remove removes pod from index if it present or returns otherwise.
func (i *PodIndex) Remove(id string) error {
	err := i.indx.Delete(id)
	if err == truncindex.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not remove pod: %v", err)
	}
	return nil
}

// Add adds the given pod. If pod already exists it returns an error.
func (i *PodIndex) Add(pod *kube.Pod) error {
	err := i.indx.Add(pod.ID(), pod)
	if err != nil {
		return fmt.Errorf("could not add pod: %v", err)
	}
	return nil
}

// Iterate calls handler func on each pod registered in index.
func (i *PodIndex) Iterate(handler func(*kube.Pod)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*kube.Pod))
	}
	i.indx.Iterate(innerIterate)
}
