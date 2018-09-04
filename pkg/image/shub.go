package image

import (
	"fmt"
	"strings"

	shub "github.com/singularityware/singularity/src/pkg/shub/client"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type shubImageInfo struct {
	ref       string
	user      string
	container string
	tags      []string
}

func parseShubRef(ref string) (shubImageInfo, error) {
	ref = strings.TrimPrefix(ref, "shub://")
	refParts := strings.Split(ref, "/")

	if len(refParts) < 2 {
		return shubImageInfo{}, fmt.Errorf("not a valid shub reference")
	}

	info := shubImageInfo{
		ref:       "shub://" + ref,
		user:      refParts[len(refParts)-2],
		container: refParts[len(refParts)-1],
		tags:      []string{ref},
	}

	info.container = strings.Split(info.container, `:`)[0]
	return info, nil
}

func (i shubImageInfo) Remote() string {
	return i.ref
}

func (i shubImageInfo) Id() string {
	return strings.Join([]string{i.user, i.container}, "_")
}

func (i shubImageInfo) Tags() []string {
	return i.tags
}

func (i shubImageInfo) Digests() []string {
	return nil
}

func (i shubImageInfo) Pull(auth *v1alpha2.AuthConfig, path string) error {
	return shub.DownloadImage(path, i.Remote(), true)
}
