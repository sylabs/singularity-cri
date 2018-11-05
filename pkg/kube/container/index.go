package container

import (
	"fmt"

	"github.com/sylabs/cri/pkg/truncindex"
)

// Index provides a convenient and thread-safe way for storing containers.
type Index struct {
	indx *truncindex.TruncIndex
}

var (
	// ErrNotFound returned when container is not found in index.
	ErrNotFound = fmt.Errorf("container not found")
)

// NewIndex returns new Index ready to use.
func NewIndex() *Index {
	return &Index{
		indx: truncindex.NewTruncIndex(contIDLen),
	}
}

// Find searches for container by its ID or prefix. This method may return error if
// prefix is not long enough to identify container uniquely.
func (i *Index) Find(id string) (*Container, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	cont, _ := item.(*Container)
	return cont, nil
}

// Remove removes container from index if it present or does nothing otherwise.
func (i *Index) Remove(id string) error {
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
func (i *Index) Add(cont *Container) error {
	err := i.indx.Add(cont.ID(), cont)
	if err != nil {
		return fmt.Errorf("could not add container: %v", err)
	}
	return nil
}

// Iterate calls handler func on each container registered in index.
func (i *Index) Iterate(handler func(*Container)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*Container))
	}
	i.indx.Iterate(innerIterate)
}
