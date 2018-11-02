package sandbox

import (
	"fmt"

	"github.com/sylabs/cri/pkg/truncindex"
)

// Index provides a convenient and thread-safe way for storing pods.
type Index struct {
	indx *truncindex.TruncIndex
}

var (
	// ErrNotFound returned when pod is not found in index.
	ErrNotFound = fmt.Errorf("pod not found")
)

// NewIndex returns new Index ready to use.
func NewIndex() *Index {
	return &Index{
		indx: truncindex.NewTruncIndex(podIDLen),
	}
}

// Find searches for pod by its ID or prefix. This method may return error if
// prefix is not long enough to identify pod uniquely.
func (i *Index) Find(id string) (*Pod, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	pod, _ := item.(*Pod)
	return pod, nil
}

// Remove removes pod from index if it present or returns otherwise.
func (i *Index) Remove(id string) error {
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
func (i *Index) Add(pod *Pod) error {
	err := i.indx.Add(pod.ID(), pod)
	if err != nil {
		return fmt.Errorf("could not add pod: %v", err)
	}
	return nil
}

// Iterate calls handler func on each pod registered in index.
func (i *Index) Iterate(handler func(pod *Pod)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*Pod))
	}
	i.indx.Iterate(innerIterate)
}
