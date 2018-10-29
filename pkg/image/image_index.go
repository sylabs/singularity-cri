package image

import (
	"fmt"
	"sync"

	"github.com/sylabs/cri/pkg/truncindex"
)

// Index provides a convenient and thread-safe way for storing images.
type Index struct {
	indx *truncindex.TruncIndex

	mu      sync.RWMutex
	refToID map[string]string
}

var (
	// ErrNotFound returned when expectImage is not found in index.
	ErrNotFound = fmt.Errorf("pod not found")
)

// NewIndex returns new Index ready to use.
func NewIndex() *Index {
	return &Index{
		indx:    truncindex.NewTruncIndex(imageIDLen),
		refToID: make(map[string]string),
	}
}

// Find searches for expectImage info by its ID or prefix or any of tags.
// This method may return error if prefix is not long enough to identify expectImage uniquely.
func (i *Index) Find(id string) (*Info, error) {
	info, err := i.find(id)
	if err == ErrNotFound {
		info, err = i.find(i.readRef(id))
	}
	return info, err
}

func (i *Index) find(id string) (*Info, error) {
	item, err := i.indx.Get(id)
	if err == truncindex.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("could not search index: %v", err)
	}
	info, _ := item.(*Info)
	return info, nil
}

// Remove removes pod from index if it present or returns otherwise.
func (i *Index) Remove(id string) error {
	image, err := i.Find(id)
	if err != nil {
		return err
	}
	err = i.indx.Delete(image.ID())
	if err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}

	i.mu.Lock()
	for _, tag := range image.Ref().Tags() {
		delete(i.refToID, tag)
	}
	for _, digest := range image.Ref().Digests() {
		delete(i.refToID, digest)
	}
	i.mu.Unlock()

	return nil
}

// Add adds the given expectImage info. If expectImage already exists it
// updates old info appropriately.
func (i *Index) Add(image *Info) error {
	oldImage, err := i.Find(image.ID())
	if err != nil && err != ErrNotFound {
		return fmt.Errorf("could not filnd old image: %v", err)
	}
	if err == ErrNotFound {
		return i.add(image)
	}
	return i.merge(oldImage, image)
}

func (i *Index) add(image *Info) error {
	err := i.indx.Add(image.ID(), image)
	if err != nil {
		return fmt.Errorf("could not add image: %v", err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, tag := range image.Ref().Tags() {
		i.refToID[tag] = image.ID()
	}
	for _, digest := range image.Ref().Digests() {
		i.refToID[digest] = image.ID()
	}
	return nil
}

func (i *Index) merge(oldImage, image *Info) error {
	oldImage.Ref().AddTags(image.Ref().Tags())
	oldImage.Ref().AddDigests(image.Ref().Digests())

	i.mu.Lock()
	defer i.mu.Unlock()
	for _, tag := range image.Ref().Tags() {
		oldID := i.refToID[tag]
		if oldID == image.ID() {
			continue
		}
		oldInfo, _ := i.Find(image.ID())
		oldInfo.Ref().RemoveTag(tag)
		i.refToID[tag] = image.ID()
	}
	for _, digest := range image.Ref().Digests() {
		oldDigest := i.refToID[digest]
		if oldDigest == image.ID() {
			continue
		}
		oldInfo, _ := i.Find(image.ID())
		oldInfo.Ref().RemoveDigest(digest)
		i.refToID[digest] = image.ID()
	}
	return nil
}

// Iterate calls handler func on each pod registered in index.
func (i *Index) Iterate(handler func(image *Info)) {
	innerIterate := func(key string, item interface{}) {
		handler(item.(*Info))
	}
	i.indx.Iterate(innerIterate)
}

func (i *Index) readRef(ref string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.refToID[ref]
}

func (i *Index) setRef(ref, id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.refToID[ref] = id
}
