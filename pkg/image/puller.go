package image

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"

	library "github.com/singularityware/singularity/src/pkg/library/client"
	shub "github.com/singularityware/singularity/src/pkg/shub/client"
	"github.com/sylabs/cri/pkg/singularity"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type imageInfo struct {
	Origin  string
	Tags    []string
	Digests []string
	Size    uint64
}

func parseImageRef(ref string) (imageInfo, error) {
	uri := singularity.DockerProtocol
	image := ref
	indx := strings.Index(ref, "://")
	if indx != -1 {
		uri = image[:indx]
		image = image[indx+3:]
	}

	info := imageInfo{
		Origin: uri,
		Tags:   []string{image},
	}
	switch uri {
	case singularity.LibraryProtocol:
	case singularity.ShubProtocol:
	case singularity.DockerProtocol:
	default:
		return imageInfo{}, fmt.Errorf("unknown image registry: %s", uri)
	}
	return info, nil
}

func pullImage(_ *k8s.AuthConfig, path string, image imageInfo) error {
	remote := fmt.Sprintf("%s://%s", image.Origin, image.Tags[0])

	switch uri := image.Origin; uri {
	case singularity.LibraryProtocol:
		return library.DownloadImage(path, remote, singularity.LibraryURL, true, "")
	case singularity.ShubProtocol:
		return shub.DownloadImage(path, remote, true)
	case singularity.DockerProtocol:
		buildCmd := exec.Command(singularity.RuntimeName, "build", path, remote)
		return buildCmd.Run()
	default:
		return fmt.Errorf("unknown image registry: %s", uri)
	}
}

func randomString(n int) string {
	buf := make([]byte, n)
	rand.Read(buf)
	return fmt.Sprintf("%x", buf)
}

func removeOrLog(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Printf("could not remove temparary image file: %v", err)
	}
}

func mergeStrSlice(t1, t2 []string) []string {
	unique := make(map[string]struct{})
	for _, tag := range append(t1, t2...) {
		unique[tag] = struct{}{}
	}
	merged := make([]string, 0, len(unique))
	for str := range unique {
		merged = append(merged, str)
	}
	return merged
}
