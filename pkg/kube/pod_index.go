package kube

import (
	"fmt"

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
		indx: truncindex.NewTruncIndex(podIDLen),
	}
}

// Find searches for pod by its ID or prefix. This method may return error if
// prefix is not long enough to identify pod uniquely.
func (i *PodIndex) Find(id string) (*Pod, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrPodNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	pod, _ := item.(*Pod)
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
func (i *PodIndex) Add(pod *Pod) error {
	err := i.indx.Add(pod.ID(), pod)
	if err != nil {
		return fmt.Errorf("could not add pod: %v", err)
	}
	return nil
}

// Iterate calls handler func on each pod registered in index.
func (i *PodIndex) Iterate(handler func(pod *Pod)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*Pod))
	}
	i.indx.Iterate(innerIterate)
}
