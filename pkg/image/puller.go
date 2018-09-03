package image

import (
	"fmt"
	"strings"

	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type imageInfo interface {
	Remote() string
	Id() string
	Tags() []string
	Digests() []string

	Pull(auth *k8s.AuthConfig, dir string) error
}

func parseImageRef(ref string) (imageInfo, error) {
	uri := "docker"
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		uri = image[:indx]
		image = image[indx+3:]
	}

	var info imageInfo
	switch uri {
	case "library":
		info = parseLibraryRef(image)
	case "shub":
		var err error
		info, err = parseShubRef(image)
		if err != nil {
			return nil, fmt.Errorf("could not parse shub ref: %v", err)
		}
	case "docker":
		info = parseOCIRef(image)
	default:
		return nil, fmt.Errorf("unknown image registry: %s", uri)
	}
	return info, nil
}
