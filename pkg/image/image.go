package image

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bytes"
	"io/ioutil"

	"github.com/sylabs/cri/pkg/rand"
	"github.com/sylabs/cri/pkg/singularity"
	"github.com/sylabs/sif/pkg/sif"
	library "github.com/sylabs/singularity/src/pkg/client/library"
	shub "github.com/sylabs/singularity/src/pkg/client/shub"
	"github.com/sylabs/singularity/src/pkg/signing"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	imageIDLen = 64
)

// Info represents image stored on host filesystem.
type Info struct {
	id     string
	sha256 string
	size   uint64
	path   string
	ref    *Reference
}

// Path returns path to image file.
func (i *Info) Path() string {
	if i == nil {
		return ""
	}
	return i.path
}

// Size returns image size in bytes.
func (i *Info) Size() uint64 {
	return i.size
}

// Ref returns associated image reference.,
func (i *Info) Ref() *Reference {
	return i.ref
}

// ID returns id of an image.
func (i *Info) ID() string {
	if i == nil {
		return ""
	}
	return i.id
}

// Pull pulls image referenced by ref and saves it to the passed location.
func Pull(location string, ref *Reference) (img *Info, err error) {
	pullPath := filepath.Join(location, "."+rand.GenerateID(64))
	defer func() {
		if err != nil {
			if err := os.Remove(pullPath); err != nil {
				log.Printf("could not remove temparary image file: %v", err)
			}
		}
	}()

	var pullURL string
	if len(ref.tags) > 0 {
		pullURL = ref.tags[0]
	} else {
		pullURL = ref.digests[0]
	}

	switch ref.uri {
	case singularity.LibraryProtocol:
		err = library.DownloadImage(pullPath, pullURL, singularity.LibraryURL, true, "")
	case singularity.ShubProtocol:
		err = shub.DownloadImage(pullPath, pullURL, true)
	case singularity.DockerProtocol:
		remote := fmt.Sprintf("%s://%s", ref.uri, pullURL)
		var errMsg bytes.Buffer
		buildCmd := exec.Command(singularity.RuntimeName, "build", "-F", pullPath, remote)
		buildCmd.Stderr = &errMsg
		buildCmd.Stdout = ioutil.Discard
		err = buildCmd.Run()
		if err != nil {
			err = fmt.Errorf("could not build image: %s", &errMsg)
		}
	default:
		err = fmt.Errorf("unknown image registry: %s", ref.uri)
	}
	if err != nil {
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	pulled, err := os.Open(pullPath)
	if err != nil {
		return nil, fmt.Errorf("could not open pulled image: %v", err)
	}

	fi, err := pulled.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not fetch file info: %v", err)
	}

	h := sha256.New()
	_, err = io.Copy(h, pulled)
	if err != nil {
		return nil, fmt.Errorf("could not get pulled image digest: %v", err)
	}
	checksum := fmt.Sprintf("%x", h.Sum(nil))

	path := filepath.Join(location, checksum)
	err = os.Rename(pullPath, path)
	if err != nil {
		return nil, fmt.Errorf("could not save pulled image: %v", err)
	}

	return &Info{
		id:     checksum,
		sha256: checksum,
		size:   uint64(fi.Size()),
		path:   path,
		ref:    ref,
	}, err
}

// Remove removes image from the host filesystem.
func (i *Info) Remove() error {
	err := os.Remove(i.path)
	if err != nil {
		return fmt.Errorf("could not remove image: %v", err)
	}
	return nil
}

// Verify versifies image signatures.
func (i *Info) Verify() error {
	fimg, err := sif.LoadContainer(i.path, true)
	if err != nil {
		return fmt.Errorf("failed to load SIF image: %v", err)
	}
	defer fimg.UnloadContainer()

	for _, desc := range fimg.DescrArr {
		err := signing.Verify(i.path, singularity.KeysServer, desc.ID, false, "")
		if err != nil {
			return fmt.Errorf("SIF verification failed: %v", err)
		}
	}
	return nil
}

// Matches tests image against passed filter and returns true if it matches.
func (i *Info) Matches(filter *k8s.ImageFilter) bool {
	if filter == nil || filter.Image == nil {
		return true
	}
	ref := filter.Image.Image
	if strings.HasPrefix(i.ID(), ref) {
		return true
	}
	for _, tag := range i.ref.tags {
		if strings.HasPrefix(tag, ref) {
			return true
		}
	}
	for _, digest := range i.ref.digests {
		if strings.HasPrefix(digest, ref) {
			return true
		}
	}
	return false
}
