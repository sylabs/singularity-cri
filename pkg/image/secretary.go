package image

import "fmt"

// ErrNotFound is returned by Secretary when image was not
// found during a Find call.
var ErrNotFound = fmt.Errorf("image not found")

// Secretary defines additional interface that should be used by
// anyone who relies on image presence, e.g. container will 'borrow'
// an image for its lifetime and 'return' at the end.
// Secretary may be used to prevent image from removal.
type Secretary interface {
	// Find searches for image by id or reference. When image is
	// not found ErrNotFound is returned.
	Find(id string) (*Info, error)
	// Borrow notifies that image is used by someone and should not be
	// removed until Return with the same parameters is called.
	Borrow(id, who string) error
	// Return notifies that image is no longer used by object who and
	// may be safely removed if no one else needs it anymore.
	Return(id, who string) error
}
