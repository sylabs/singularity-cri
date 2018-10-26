package image

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sylabs/cri/pkg/rand"
	"github.com/sylabs/cri/pkg/singularity"
	library "github.com/sylabs/singularity/src/pkg/client/library"
	shub "github.com/sylabs/singularity/src/pkg/client/shub"
)

type Reference struct {
	uri     string
	tags    []string
	digests []string
}

type Image struct {
	checksum string
	size     uint64
	ref      Reference
}

func ParseRef(ref string) (Reference, error) {
	uri := singularity.DockerProtocol
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		uri = image[:indx]
		image = image[indx+3:]
	}

	info := Reference{
		uri: uri,
	}

	switch uri {
	case singularity.ShubProtocol:
		fallthrough
	case singularity.LibraryProtocol:
		if strings.Contains(image, "sha256.") {
			info.digests = append(info.digests, ref)
		} else {
			info.tags = append(info.tags, normalizedImageRef(ref))
		}
	case singularity.DockerProtocol:
		if strings.IndexByte(image, '@') != -1 {
			info.digests = append(info.digests, image)
		} else {
			info.tags = append(info.tags, normalizedImageRef(image))
		}
	default:
		return Reference{}, fmt.Errorf("unknown image registry: %s", uri)
	}

	return info, nil
}

func Pull(location string, ref Reference) (*Image, error) {
	//randID := randomString()
	//pullPath := s.pullPath(randID)

	var pullURL string
	if len(ref.tags) > 0 {
		pullURL = ref.tags[0]
	} else {
		pullURL = ref.digests[0]
	}

	var err error
	switch ref.uri {
	case singularity.LibraryProtocol:
		err = library.DownloadImage(location, pullURL, singularity.LibraryURL, true, "")
	case singularity.ShubProtocol:
		err = shub.DownloadImage(location, pullURL, true)
	case singularity.DockerProtocol:
		remote := fmt.Sprintf("%s://%s", ref.uri, ref)
		buildCmd := exec.Command(singularity.RuntimeName, "build", "-F", location, remote)
		err = buildCmd.Run()
	default:
		err = fmt.Errorf("unknown image registry: %s", ref.uri)
	}
	if err != nil {
		return nil, fmt.Errorf("could not pull image: %v", err)
	}

	return &Image{
		checksum: "",
		size:     0,
		ref:      ref,
	}, err
}

// normalizedImageRef appends tag 'latest' if the passed ref
// does not have any tag or digest already.
func normalizedImageRef(ref string) string {
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		image = ref[indx+3:]
	}
	i := strings.LastIndexByte(image, ':')
	if i == -1 {
		return ref + ":latest"
	}
	return ref
}
