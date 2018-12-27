package image

import (
	"fmt"
	"sync"

	"github.com/sylabs/cri/pkg/image"
	"github.com/sylabs/cri/pkg/index"
	"github.com/sylabs/cri/pkg/slice"
)

var errIsBorrowed = fmt.Errorf("image is used by at least one container")

type shelf struct {
	indx *index.ImageIndex

	mu     sync.RWMutex
	ledger map[string][]string
}

func newShelf() *shelf {
	return &shelf{
		indx:   index.NewImageIndex(),
		ledger: make(map[string][]string),
	}
}

// Find looks for image in index by id or reference. When image
// is not found in index image.ErrNotFound is returned.
// This method is thread-safe to use.
func (s *shelf) Find(id string) (*image.Info, error) {
	img, err := s.indx.Find(id)
	if err == index.ErrNotFound {
		return nil, image.ErrNotFound
	}
	return img, nil
}

// Borrow notifies that image is used by some container and should
// not be removed until Return with the same parameters is called.
// This method is thread-safe to use.
func (s *shelf) Borrow(id, who string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// double check image is there
	info, err := s.Find(id)
	if err != nil {
		return fmt.Errorf("could not borrow image: %v", err)
	}

	tracked := s.ledger[info.ID()]
	tracked = slice.MergeString(tracked, who)
	s.ledger[info.ID()] = tracked
	return nil
}

// Return notifies that image is no longer used by a container and
// may be safely removed if no one else needs it anymore.
// This method is thread-safe to use.
func (s *shelf) Return(id, who string) error {
	// double check image is there
	info, err := s.Find(id)
	if err != nil {
		return fmt.Errorf("could not return image: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tracked := s.ledger[info.ID()]
	tracked = slice.RemoveFromString(tracked, who)
	if len(tracked) == 0 {
		delete(s.ledger, info.ID())
	} else {
		s.ledger[info.ID()] = tracked
	}
	return nil
}

func (s *shelf) status(id string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tracked := s.ledger[id]
	return tracked, nil
}

func (s *shelf) add(img *image.Info) error {
	return s.indx.Add(img)
}

func (s *shelf) list() ([]*image.Info, error) {
	var images []*image.Info
	addToList := func(img *image.Info) {
		images = append(images, img)
	}
	s.indx.Iterate(addToList)
	return images, nil
}

func (s *shelf) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// double check no one borrowed image
	_, ok := s.ledger[id]
	if ok {
		return errIsBorrowed
	}

	delete(s.ledger, id)
	err := s.indx.Remove(id)
	if err != nil {

	}
	return nil
}
